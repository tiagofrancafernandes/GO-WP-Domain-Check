package main

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "regexp"
    "strconv"
    "strings"
    "time"
)

type Proxy struct {
    Host     string
    Port     string
    Username string
    Password string
    Type     string
    Active   bool
}

type DomainResult struct {
    Domain           string            `json:"domain"`
    StatusCode       int               `json:"status_code"`
    IsWordPress      bool              `json:"is_wordpress"`
    WPVersion        string            `json:"wp_version,omitempty"`
    WPTheme          string            `json:"wp_theme,omitempty"`
    WPPlugins        []string          `json:"wp_plugins,omitempty"`
    Headers          map[string]string `json:"headers,omitempty"`
    Error            string            `json:"error,omitempty"`
    ProxyUsed        string            `json:"proxy_used,omitempty"`
    RedirectLocation string            `json:"redirect_location,omitempty"`
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: go run main.go <domain>")
        return
    }

    domain := os.Args[1]
    if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
        domain = "https://" + domain
    }

    result := DomainResult{
        Domain: domain,
    }

    // Primeiro tenta sem proxy
    statusCode, body, headers, err := checkDomain(domain, nil)
    if err != nil {
        result.Error = err.Error()
        outputJSON(result)
        return
    }

    result.StatusCode = statusCode
    result.Headers = headers

    // Verifica redirecionamento
    if location, ok := headers["Location"]; ok && (statusCode == 301 || statusCode == 302) {
        result.RedirectLocation = location
    }

    // Se não for 403, processa o resultado
    if statusCode != 403 {
        processResult(&result, body)
        outputJSON(result)
        return
    }

    // Se for 403, tenta com proxies
    proxies, err := loadProxies("proxies.csv")
    if err != nil {
        result.Error = fmt.Sprintf("Failed to load proxies: %s", err)
        outputJSON(result)
        return
    }

    for i, proxy := range proxies {
        if !proxy.Active {
            continue
        }

        statusCode, body, headers, err := checkDomain(domain, &proxy)
        if err != nil {
            // Marcar proxy como inativo
            markProxyAsInactive(proxies, i, "proxies.csv")
            continue
        }

        result.StatusCode = statusCode
        result.Headers = headers
        result.ProxyUsed = fmt.Sprintf("%s:%s", proxy.Host, proxy.Port)

        // Verifica redirecionamento
        if location, ok := headers["Location"]; ok && (statusCode == 301 || statusCode == 302) {
            result.RedirectLocation = location
        }

        // Processa o resultado obtido via proxy
        processResult(&result, body)
        outputJSON(result)
        return
    }

    // Se chegou aqui, é porque todos os proxies falharam ou ainda retornam 403
    result.StatusCode = 403
    result.Error = "All proxies failed or returned 403"
    outputJSON(result)
}

func processResult(result *DomainResult, body string) {
    // Verifica se é WordPress e extrai informações
    isWP, wpInfo := detectWordPress(body)
    result.IsWordPress = isWP

    if isWP {
        result.WPVersion = wpInfo.Version
        result.WPTheme = wpInfo.Theme
        result.WPPlugins = wpInfo.Plugins
    }
}

type WordPressInfo struct {
    Version string
    Theme   string
    Plugins []string
}

func detectWordPress(body string) (bool, WordPressInfo) {
    info := WordPressInfo{}

    // Indicadores de que o site é WordPress
    wpIndicators := []string{
        "/wp-content/",
        "/wp-includes/",
        "wp-login.php",
        "wp-admin",
    }

    isWP := false
    for _, indicator := range wpIndicators {
        if strings.Contains(body, indicator) {
            isWP = true
            break
        }
    }

    if !isWP {
        return false, info
    }

    // Extrai a versão do WordPress
    versionPatterns := []*regexp.Regexp{
        regexp.MustCompile(`<meta name="generator" content="WordPress ([0-9.]+)`),
        regexp.MustCompile(`ver=([0-9.]+)`),
        regexp.MustCompile(`wp-includes/js/wp-emoji-release.min.js\?ver=([0-9.]+)`),
    }

    for _, pattern := range versionPatterns {
        matches := pattern.FindStringSubmatch(body)
        if len(matches) > 1 {
            info.Version = matches[1]
            break
        }
    }

    // Extrai o tema do WordPress
    themePattern := regexp.MustCompile(`/wp-content/themes/([^/]+)`)
    themeMatches := themePattern.FindStringSubmatch(body)
    if len(themeMatches) > 1 {
        info.Theme = themeMatches[1]
    }

    // Extrai plugins do WordPress
    pluginPattern := regexp.MustCompile(`/wp-content/plugins/([^/]+)`)
    pluginMatches := pluginPattern.FindAllStringSubmatch(body, -1)

    pluginsMap := make(map[string]bool) // Para evitar duplicatas
    for _, match := range pluginMatches {
        if len(match) > 1 {
            pluginsMap[match[1]] = true
        }
    }

    for plugin := range pluginsMap {
        info.Plugins = append(info.Plugins, plugin)
    }

    return true, info
}

func outputJSON(result DomainResult) {
    jsonData, err := json.MarshalIndent(result, "", "  ")
    if err != nil {
        fmt.Printf("Error generating JSON: %s\n", err)
        return
    }
    fmt.Println(string(jsonData))
}

func checkDomain(domain string, proxy *Proxy) (int, string, map[string]string, error) {
    client := &http.Client{
        Timeout: 10 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse // Não seguir redirecionamentos
        },
    }

    if proxy != nil {
        var proxyURL *url.URL
        var err error

        if proxy.Username != "" && proxy.Password != "" {
            proxyURL, err = url.Parse(fmt.Sprintf("%s://%s:%s@%s:%s",
                strings.ToLower(proxy.Type),
                proxy.Username,
                proxy.Password,
                proxy.Host,
                proxy.Port))
        } else {
            proxyURL, err = url.Parse(fmt.Sprintf("%s://%s:%s",
                strings.ToLower(proxy.Type),
                proxy.Host,
                proxy.Port))
        }

        if err != nil {
            return 0, "", nil, fmt.Errorf("invalid proxy URL: %v", err)
        }

        client.Transport = &http.Transport{
            Proxy: http.ProxyURL(proxyURL),
        }
    }

    req, err := http.NewRequest("GET", domain, nil)
    if err != nil {
        return 0, "", nil, err
    }

    // Adicionar User-Agent para evitar bloqueios
    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

    resp, err := client.Do(req)
    if err != nil {
        return 0, "", nil, err
    }
    defer resp.Body.Close()

    // Extrair headers
    headers := make(map[string]string)
    for name, values := range resp.Header {
        if len(values) > 0 {
            headers[name] = values[0]
        }
    }

    // Ler o corpo da resposta
    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return resp.StatusCode, "", headers, err
    }

    return resp.StatusCode, string(bodyBytes), headers, nil
}

func loadProxies(filename string) ([]Proxy, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    // Pular cabeçalho
    _, err = reader.Read()
    if err != nil {
        return nil, err
    }

    var proxies []Proxy
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }

        // Assumindo formato: host,port,username,password,type,active
        if len(record) < 6 {
            continue
        }

        active, _ := strconv.ParseBool(record[5])
        proxy := Proxy{
            Host:     record[0],
            Port:     record[1],
            Username: record[2],
            Password: record[3],
            Type:     record[4],
            Active:   active,
        }
        proxies = append(proxies, proxy)
    }

    return proxies, nil
}

func markProxyAsInactive(proxies []Proxy, index int, filename string) error {
    // Marcar como inativo na memória
    proxies[index].Active = false

    // Abrir arquivo para leitura
    file, err := os.Open(filename)
    if err != nil {
        return err
    }

    // Ler todas as linhas
    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        file.Close()
        return err
    }
    file.Close()

    // Atualizar a linha correspondente (índice + 1 por causa do cabeçalho)
    if len(records) > index+1 {
        records[index+1][5] = "false"
    }

    // Escrever de volta para o arquivo
    outFile, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer outFile.Close()

    writer := csv.NewWriter(outFile)
    err = writer.WriteAll(records)
    if err != nil {
        return err
    }

    return nil
}
