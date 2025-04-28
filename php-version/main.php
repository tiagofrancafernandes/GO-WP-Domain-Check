<?php
/**
 * Domain Checker with Proxy Support
 *
 * This script checks a domain for WordPress detection and other information.
 * It uses proxies when encountering 403 errors.
 *
 * Usage via CLI: php domain-check.php example.com
 * Usage as library: include this file and call checkDomain()
 */

/**
 * Main function to check a domain
 *
 * @param string $domain Domain to check
 * @param bool $returnJson Whether to return JSON (true) or array (false)
 * @return string|array Result as JSON string or array
 */
function checkDomain($domain, $returnJson = false)
{
    // Ensure domain has protocol
    if (!preg_match('~^https?://~i', $domain)) {
        $domain = 'https://' . $domain;
    }

    $result = [
        'domain' => $domain,
        'status_code' => null,
        'is_wordpress' => false,
        'wp_version' => null,
        'wp_theme' => null,
        'wp_plugins' => [],
        'headers' => [],
        'error' => null,
        'proxy_used' => null,
        'redirect_location' => null
    ];

    // First try without proxy
    list($statusCode, $body, $headers) = fetchUrl($domain);

    if ($statusCode === false) {
        $result['error'] = "Connection error: " . $body;
        return $returnJson ? json_encode($result, JSON_PRETTY_PRINT) : $result;
    }

    $result['status_code'] = $statusCode;
    $result['headers'] = $headers;

    // Check for redirect
    if (($statusCode == 301 || $statusCode == 302) && isset($headers['location'])) {
        $result['redirect_location'] = $headers['location'];
    }

    // If not 403, process the result
    if ($statusCode != 403) {
        processWordPressData($result, $body);
        return $returnJson ? json_encode($result, JSON_PRETTY_PRINT) : $result;
    }

    // If 403, try with proxies
    $proxies = loadProxies('proxies.csv');
    if (empty($proxies)) {
        $result['error'] = "No proxies available";
        return $returnJson ? json_encode($result, JSON_PRETTY_PRINT) : $result;
    }

    foreach ($proxies as $index => $proxy) {
        if ($proxy['active'] !== true) {
            continue;
        }

        list($statusCode, $body, $headers) = fetchUrl($domain, $proxy);

        if ($statusCode === false) {
            // Mark proxy as inactive
            markProxyAsInactive($proxies, $index);
            continue;
        }

        $result['status_code'] = $statusCode;
        $result['headers'] = $headers;
        $result['proxy_used'] = "{$proxy['host']}:{$proxy['port']}";

        // Check for redirect
        if (($statusCode == 301 || $statusCode == 302) && isset($headers['location'])) {
            $result['redirect_location'] = $headers['location'];
        }

        // Process the result obtained via proxy
        processWordPressData($result, $body);
        return $returnJson ? json_encode($result, JSON_PRETTY_PRINT) : $result;
    }

    // If we get here, all proxies failed or still return 403
    $result['status_code'] = 403;
    $result['error'] = "All proxies failed or returned 403";

    return $returnJson ? json_encode($result, JSON_PRETTY_PRINT) : $result;
}

/**
 * Fetch URL with optional proxy
 *
 * @param string $url URL to fetch
 * @param array|null $proxy Proxy configuration
 * @return array [status_code, body, headers]
 */
function fetchUrl($url, $proxy = null)
{
    $ch = curl_init();

    curl_setopt($ch, CURLOPT_URL, $url);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_HEADER, true);
    curl_setopt($ch, CURLOPT_FOLLOWLOCATION, false);
    curl_setopt($ch, CURLOPT_TIMEOUT, 10);
    curl_setopt($ch, CURLOPT_USERAGENT, 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36');

    // Set proxy if provided
    if ($proxy !== null) {
        $proxyAuth = '';
        if (!empty($proxy['username']) && !empty($proxy['password'])) {
            $proxyAuth = $proxy['username'] . ':' . $proxy['password'];
        }

        curl_setopt($ch, CURLOPT_PROXY, $proxy['host']);
        curl_setopt($ch, CURLOPT_PROXYPORT, $proxy['port']);

        if (!empty($proxyAuth)) {
            curl_setopt($ch, CURLOPT_PROXYUSERPWD, $proxyAuth);
        }

        // Set proxy type
        $proxyType = strtolower($proxy['type']);
        if ($proxyType === 'http') {
            curl_setopt($ch, CURLOPT_PROXYTYPE, CURLPROXY_HTTP);
        } elseif ($proxyType === 'https') {
            curl_setopt($ch, CURLOPT_PROXYTYPE, CURLPROXY_HTTP);
            curl_setopt($ch, CURLOPT_HTTPPROXYTUNNEL, true);
        } elseif ($proxyType === 'socks4') {
            curl_setopt($ch, CURLOPT_PROXYTYPE, CURLPROXY_SOCKS4);
        } elseif ($proxyType === 'socks5') {
            curl_setopt($ch, CURLOPT_PROXYTYPE, CURLPROXY_SOCKS5);
        }
    }

    $response = curl_exec($ch);

    if ($response === false) {
        $error = curl_error($ch);
        curl_close($ch);
        return [false, $error, []];
    }

    $statusCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $headerSize = curl_getinfo($ch, CURLINFO_HEADER_SIZE);

    $headerText = substr($response, 0, $headerSize);
    $body = substr($response, $headerSize);

    curl_close($ch);

    // Parse headers
    $headers = [];
    foreach (explode("\r\n", $headerText) as $line) {
        if (preg_match('/^([^:]+):\s*(.+)$/', $line, $matches)) {
            $headers[strtolower($matches[1])] = $matches[2];
        }
    }

    return [$statusCode, $body, $headers];
}

/**
 * Process WordPress data from HTML content
 *
 * @param array &$result Result array to update
 * @param string $body HTML content
 */
function processWordPressData(&$result, $body)
{
    // Check if it's WordPress
    $wpIndicators = [
        '/wp-content/',
        '/wp-includes/',
        'wp-login.php',
        'wp-admin'
    ];

    $isWp = false;
    foreach ($wpIndicators as $indicator) {
        if (strpos($body, $indicator) !== false) {
            $isWp = true;
            break;
        }
    }

    $result['is_wordpress'] = $isWp;

    if (!$isWp) {
        return;
    }

    // Extract WordPress version
    $versionPatterns = [
        '/<meta name="generator" content="WordPress ([0-9.]+)/',
        '/ver=([0-9.]+)/',
        '/wp-includes\/js\/wp-emoji-release\.min\.js\?ver=([0-9.]+)/'
    ];

    foreach ($versionPatterns as $pattern) {
        if (preg_match($pattern, $body, $matches)) {
            $result['wp_version'] = $matches[1];
            break;
        }
    }

    // Extract WordPress theme
    if (preg_match('/\/wp-content\/themes\/([^\/]+)/', $body, $matches)) {
        $result['wp_theme'] = $matches[1];
    }

    // Extract WordPress plugins
    $plugins = [];
    preg_match_all('/\/wp-content\/plugins\/([^\/]+)/', $body, $matches);

    if (!empty($matches[1])) {
        $result['wp_plugins'] = array_values(array_unique($matches[1]));
    }
}

/**
 * Load proxies from CSV file
 *
 * @param string $filename CSV file path
 * @return array Array of proxy configurations
 */
function loadProxies($filename)
{
    if (!file_exists($filename)) {
        return [];
    }

    $proxies = [];
    $handle = fopen($filename, 'r');

    if ($handle === false) {
        return [];
    }

    // Skip header
    fgetcsv($handle);

    while (($data = fgetcsv($handle)) !== false) {
        if (count($data) < 6) {
            continue;
        }

        $proxies[] = [
            'host' => $data[0],
            'port' => $data[1],
            'username' => $data[2],
            'password' => $data[3],
            'type' => $data[4],
            'active' => strtolower($data[5]) === 'true'
        ];
    }

    fclose($handle);
    return $proxies;
}

/**
 * Mark a proxy as inactive in the CSV file
 *
 * @param array $proxies Array of proxies
 * @param int $index Index of proxy to mark inactive
 * @return bool Success status
 */
function markProxyAsInactive($proxies, $index)
{
    $filename = 'proxies.csv';

    if (!file_exists($filename)) {
        return false;
    }

    // Mark as inactive in memory
    $proxies[$index]['active'] = false;

    // Read all lines
    $lines = file($filename);
    if ($lines === false) {
        return false;
    }

    // Update the corresponding line (index + 1 because of header)
    $lineIndex = $index + 1;
    if (isset($lines[$lineIndex])) {
        $data = str_getcsv($lines[$lineIndex]);
        if (count($data) >= 6) {
            $data[5] = 'false';
            $lines[$lineIndex] = implode(',', $data) . "\n";
        }
    }

    // Write back to file
    return file_put_contents($filename, implode('', $lines)) !== false;
}

/**
 * CLI entry point
 */
if (PHP_SAPI === 'cli' && !empty($argv[1])) {
    echo checkDomain($argv[1], true) . PHP_EOL;
}
