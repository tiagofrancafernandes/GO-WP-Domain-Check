# Golang Wordpress Checker

#### Instalação do GO
- [go.dev/doc/install](https://go.dev/doc/install)

#### Executar o fonte

```sh
go run main.go
```

Múltiplas domínios

```sh
go run main.go domain.com seconddomain.com
```

#### Compilação (Geração do binário)

##### Compilação básica

```sh
go build -o wordpress-checker main.go
```
> Este comando irá compilar o arquivo `main.go` e criar um executável chamado `wordpress-checker` no mesmo diretório.

##### Compilação para diferentes sistemas operacionais
Se você precisar compilar para diferentes sistemas operacionais, pode usar as variáveis de ambiente GOOS e GOARCH:

* Para Windows:
```sh
GOOS=windows GOARCH=amd64 go build -o wordpress-checker-windows-amd64.exe main.go
```

* Para Linux:
```sh
GOOS=linux GOARCH=amd64 go build -o wordpress-checker-linux-amd64 main.go
```

* Para macOS:

Chip Intel:
```sh
GOOS=darwin GOARCH=amd64 go build -o wordpress-checker_macos_amd64 main.go
```

Apple Silicon (M1/M2):
```sh
GOOS=darwin GOARCH=arm64 go build -o wordpress-checker_macos_arm64 main.go
```

--------------
###### Shell Scripts

Ou pode usar os scripts shell abaixo:

* Para Windows:
```sh
./build-windows-wsl.sh ## output/windows/wordpress-checker-windows-amd64.exe
```
* Para Linux:
```sh
./build-linux.sh ## output/linux/wordpress-checker-linux-amd64
```
* Para macOS:
```sh
./build-mac-darwin-amd64.sh ## output/macos/wordpress-checker-darwin-amd64
```

```sh
./build-mac-darwin-arm64.sh ## output/macos/wordpress-checker-darwin-arm64
```


A saída do executável estará no diretório `output/[SO]` onde SO é o sistema operacional.
```
output
├── linux/wordpress-checker-linux-amd64
├── macos
│   └── wordpress-checker-darwin-amd64
│   └── wordpress-checker-darwin-arm64
└── windows
    └── wordpress-checker-windows-amd64.exe
```

* Otimização do executável
Para criar um executável otimizado (menor e mais rápido):

```sh
go build -ldflags="-s -w" -o wordpress-checker main.go
```

As flags `-s -w` removem informações de depuração e tabelas de símbolos, reduzindo o tamanho do executável.

Compilação com todas as dependências (build estático)
Para criar um executável que inclui todas as dependências:

```sh
CGO_ENABLED=0 go build -o wordpress-checker main.go
```

Isso é útil para garantir que o executável funcione em sistemas que não têm o Go instalado.

* Verificação do executável

Após a compilação, você pode verificar se o executável foi criado corretamente:

```sh
ls -la wordpress-checker
```


E para executá-lo:
```sh
./wordpress-checker domain.com
```

Ou com múltiplas domínios:
```sh
./wordpress-checker domain.com seconddomain.com
```

#### Exemplos de saída
```sh
go run main.go domain.com
```
Saída
```
[
  {
    "domain": "domain.com",
    "final_url": "https://www.domain.com/",
    "is_wordpress": false,
    "wordpress_version": "",
    "wordpress_evidences": "",
    "errors": [
      "status code 403"
    ]
  }
]
```
-----

```sh
go run main.go wordpress.com
```
Saída
```
[
  {
    "domain": "wordpress.com",
    "final_url": "https://wordpress.com",
    "is_wordpress": true,
    "wordpress_version": "Unknown",
    "wordpress_evidences": "wp-content",
    "errors": []
  }
]
```
