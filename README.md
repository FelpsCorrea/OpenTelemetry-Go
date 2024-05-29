# OpenTelemetry-Go

This repository is a web API developed in Golang using microservices as a partial assessment for the completion of the Postgraduate Degree in Golang.

In this project, concepts such as the following were utilized:
- Docker
- Open Telemetry: Capture and export telemetry data (metrics, logs, and traces)
- Zipkin: A distributed tracing system that helps gather timing data needed to troubleshoot latency problems in service architectures
- External API Calls: Utilized the [ViaCep](https://viacep.com.br) API and [Weather](https://www.weatherapi.com) API

To run this project, you need to start the Docker environment using the following command:
```sh
docker-compose up -d --build
```

So, you can make calls using the following URL:
```
http://localhost:8080/weather/YOUR_CEP
```

To see the tracing in Zipkin you can access the following URL:
```
http://localhost:9411
```