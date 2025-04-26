package main

import (
    "crypto/tls"
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "net"
    "net/http"
    // "net/url"
    // "os"
    "regexp"
    // "strconv"
    "strings"
    "sync"
    "time"
)

type Result struct {
    Domain            string   `json:"domain"`
    DomainIsValid     bool     `json:"domain_is_valid"`
    DomainHasDNSRecord bool    `json:"domain_has_dns_record"`
    FinalURL          string   `json:"final_url"`
    IsWordPress       bool     `json:"is_wordpress"`
    WordPressVersion  string   `json:"wordpress_version"`
    WordPressEvidences string  `json:"wordpress_evidences"`
    ResponseTime      string   `json:"response_time"`
    Errors            []string `json:"errors"`
}

func main() {
    maxConcurrency := flag.Int("max_concurrency", 5, "Maximum number of concurrent requests")
    timeout := flag.Int("timeout", 10, "Request timeout in seconds")
    flag.Parse()

    if *maxConcurrency < 1 {
        fmt.Println("Invalid max concurrency value. Must be greater than or equal to 1.")
        return
    }

    if *timeout < 1 {
        fmt.Println("Invalid timeout value. Must be greater than or equal to 1.")
        return
    }

    domains := flag.Args()
    if len(domains) == 0 {
        fmt.Println("Usage: go run main.go --max_concurrency <max_concurrency> --timeout <timeout> <domain1> <domain2> ...")
        return
    }

    results := processDomainsConcurrently(domains, *maxConcurrency, *timeout)

    jsonResult, err := json.MarshalIndent(results, "", "  ")
    if err != nil {
        fmt.Println("Error generating JSON:", err)
        return
    }

    fmt.Println(string(jsonResult))
}

func processDomainsConcurrently(domains []string, maxConcurrency, timeout int) []Result {
    var wg sync.WaitGroup
    results := make([]Result, 0, len(domains))
    resultChan := make(chan Result, len(domains))
    sem := make(chan struct{}, maxConcurrency)

    for _, domain := range domains {
        wg.Add(1)
        sem <- struct{}{} // Acquire a slot
        go func(domain string) {
            defer wg.Done()
            defer func() { <-sem }() // Release the slot
            result := checkDomain(domain, timeout)
            resultChan <- result
        }(domain)
    }

    go func() {
        wg.Wait()
        close(resultChan)
    }()

    for result := range resultChan {
        results = append(results, result)
    }

    return results
}

func checkDomain(domain string, timeout int) Result {
    result := Result{
        Domain: domain,
        DomainIsValid: false,
        DomainHasDNSRecord: false,
    }
    errors := []string{}

    // Validate domain structure
    if !isValidDomain(domain) {
        errors = append(errors, "invalid domain structure")
        result.Errors = errors
        return result
    }

    // Mark domain as valid
    result.DomainIsValid = true

    // Check if domain is registered
    if !isDomainRegistered(domain) {
        errors = append(errors, "domain not registered")
        result.Errors = errors
        return result
    }

    // Mark domain as having DNS records
    result.DomainHasDNSRecord = true

    // Make initial request
    startTime := time.Now()
    finalURL, statusCode, body, err := makeRequest(domain, false, timeout)
    responseTime := time.Since(startTime)
    result.ResponseTime = responseTime.String()

    if err != nil {
        errors = append(errors, err.Error())
    }

    // Handle SSL errors
    if err != nil && strings.Contains(err.Error(), "x509") {
        errors = append(errors, "SSL error")
        startTime = time.Now()
        finalURL, statusCode, body, err = makeRequest(domain, true, timeout)
        responseTime = time.Since(startTime)
        result.ResponseTime = responseTime.String()
        if err != nil {
            errors = append(errors, err.Error())
        }
    }

    // Check status code
    if statusCode != 200 {
        errors = append(errors, fmt.Sprintf("status code %d", statusCode))
        if statusCode == 403 {
            if isCloudflare(body) {
                errors = append(errors, "blocked by Cloudflare")
            }
        }
    }

    // Check for blank screen
    if isBlankScreen(body) {
        errors = append(errors, "blank screen")
    }

    // Check if it's a WordPress site
    isWordPress, wpVersion, wpEvidences := detectWordPress(body)
    if isWordPress {
        result.IsWordPress = true
        result.WordPressVersion = wpVersion
        result.WordPressEvidences = wpEvidences
    }

    result.FinalURL = finalURL
    result.Errors = errors
    return result
}

func isValidDomain(domain string) bool {
    // Regex para validar a estrutura do domínio
    domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
    return domainRegex.MatchString(domain)
}

func isDomainRegistered(domain string) bool {
    _, err := net.LookupHost(domain)
    return err == nil
}

func makeRequest(domain string, ignoreSSL bool, timeout int) (string, int, string, error) {
    client := &http.Client{
        Timeout: time.Duration(timeout) * time.Second,
    }
    if ignoreSSL {
        client.Transport = &http.Transport{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        }
    }

    resp, err := client.Get("https://" + domain)
    if err != nil {
        return "", 0, "", err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", resp.StatusCode, "", err
    }

    finalURL := resp.Request.URL.String()
    return finalURL, resp.StatusCode, string(body), nil
}

func isCloudflare(body string) bool {
    return strings.Contains(body, "Cloudflare")
}

func stripTags(html string) string {
    re := regexp.MustCompile(`<[^>]*>`)
    return re.ReplaceAllString(html, "")
}

func isBlankScreen(body string) bool {
    cleanedBody := stripTags(body)
    return strings.TrimSpace(cleanedBody) == ""
}

// Função para validar se uma versão está no formato correto (X.Y ou X.Y.Z)
// onde X é de 4 a 9, Y e Z são de 0 a 99
func isValidVersion(version string) bool {
    // Regex para validar o formato X.Y ou X.Y.Z
    validVersionRegex := regexp.MustCompile(`^[4-9]\.\d{1,2}(\.\d{1,2})?$`)
    return validVersionRegex.MatchString(version)
}

func detectWordPress(body string) (bool, string, string) {
    bodyLower := strings.ToLower(body)

    // Evidências de que é WordPress
    evidences := []string{}

    if strings.Contains(bodyLower, "wp-content") {
        evidences = append(evidences, "wp-content")
    }

    if strings.Contains(bodyLower, "wp-includes") {
        evidences = append(evidences, "wp-includes")
    }

    if strings.Contains(bodyLower, "wp-json") {
        evidences = append(evidences, "wp-json")
    }

    if strings.Contains(bodyLower, "wp-emoji") {
        evidences = append(evidences, "wp-emoji")
    }

    if strings.Contains(bodyLower, "elementor") {
        evidences = append(evidences, "elementor")
    }

    // Se não encontrou nenhuma evidência, não é WordPress
    if len(evidences) == 0 {
        return false, "", ""
    }

    // Verificar versão via meta tag
    metaRegex := regexp.MustCompile(`<meta\s+name=["']generator["']\s+content=["']WordPress\s+([0-9.]+)["']`)
    metaMatches := metaRegex.FindStringSubmatch(body)
    if len(metaMatches) > 1 && isValidVersion(metaMatches[1]) {
        return true, metaMatches[1], "meta generator: " + strings.Join(evidences, ", ")
    }

    // Verificar versão via wp-embed.min.js
    embedRegex := regexp.MustCompile(`/wp-includes/js/wp-embed\.min\.js\?ver=([0-9.]+)`)
    embedMatches := embedRegex.FindStringSubmatch(body)
    if len(embedMatches) > 1 && isValidVersion(embedMatches[1]) {
        return true, embedMatches[1], "wp-embed.min.js: " + strings.Join(evidences, ", ")
    }

    // Verificar versão via wp-emoji-release.min.js
    emojiRegex := regexp.MustCompile(`wp-emoji-release\.min\.js\?ver=([0-9.]+)`)
    emojiMatches := emojiRegex.FindStringSubmatch(body)
    if len(emojiMatches) > 1 && isValidVersion(emojiMatches[1]) {
        return true, emojiMatches[1], "wp-emoji-release.min.js: " + strings.Join(evidences, ", ")
    }

    // Verificar versão via qualquer asset com parâmetro ver
    // Agora usando regex para encontrar a versão e depois validando o formato
    verRegex := regexp.MustCompile(`\?ver=([0-9.]+)`)
    verMatches := verRegex.FindStringSubmatch(body)
    if len(verMatches) > 1 && isValidVersion(verMatches[1]) {
        return true, verMatches[1], "asset version: " + strings.Join(evidences, ", ")
    }

    // Verificar versão via meta tag do Elementor
    elementorMetaRegex := regexp.MustCompile(`<meta\s+name=["']generator["']\s+content=["']Elementor\s+([0-9.]+)["']`)
    elementorMetaMatches := elementorMetaRegex.FindStringSubmatch(body)
    if len(elementorMetaMatches) > 1 && isValidVersion(elementorMetaMatches[1]) {
        return true, elementorMetaMatches[1], "elementor meta generator: " + strings.Join(evidences, ", ")
    }

    // É WordPress, mas versão desconhecida ou não está no formato esperado
    return true, "Unknown", strings.Join(evidences, ", ")
}
