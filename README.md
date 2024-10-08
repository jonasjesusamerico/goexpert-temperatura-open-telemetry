
## Go Challenge: CEP Temperature System with OpenTelemetry

Este projeto consiste em dois serviços Go:

- **Servico A**: Recebe um código postal (CEP) via POST, valida seu formato e o encaminha ao Serviço B
- **Servico B**: Recebe um CEP válido, recupera a localização por meio do serviço ViaCEP e busca a temperatura atual usando o WeatherAPI.

### Instruções para execução do projeto

1. **Rodando Docker Compose**: Na raiz do projeto (onde o arquivo `docker-compose.yml` está localizado), execute o seguinte comando:

   ```bash
   docker-compose build; docker-compose up -d
   ```

2. **Testando a aplicação**: Depois que os serviços estiverem em execução, você pode testá-los usando os seguintes comandos:

- Para o Serviço A:

   ```bash
   curl -X POST -H "Content-Type: application/json" -d '{"cep":"88840000"}' http://localhost:8080/cep
   ```

- Para o Serviço B (substitua `CEP` por um código postal válido de 8 dígitos):

   ```bash
   curl -X GET "http://localhost:8081/clima?cep=88840000" -H "accept: application/json"
   ```

3. **Teste do Zipkin**:

- Abra o seguinte link no seu navegador:

   ```
   http://localhost:9411/zipkin/?lookback=15m&endTs=1728346711416&limit=10
   ```