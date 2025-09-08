# Dockerfile para o Serviço A

# Use uma imagem Go oficial como base
FROM golang:1.22

# Defina o diretório de trabalho dentro do contêiner
WORKDIR /app

# Copie os arquivos de dependência e baixe as dependências
COPY go.mod go.sum ./
RUN go mod download

# Copie o código-fonte
COPY . .

# Construa a aplicação
RUN CGO_ENABLED=0 go build -o /service-a ./cmd/main.go

# Exponha a porta
EXPOSE 8080

# Comando para rodar a aplicação
CMD ["/service-a"]