# Sistema de Processamento de Vídeos - FIAP X

## Introdução

Vocês foram contratados pela empresa FIAP X que precisa avançar no desenvolvimento de um projeto de processamento de imagens. Em uma rodada de investimentos, a empresa apresentou um projeto simples que processa um vídeo e retorna as imagens dele em um arquivo .zip.

Os investidores gostaram tanto do projeto, que querem investir em uma versão onde eles possam enviar um vídeo e fazer download deste zip.

## Desafio

O seu desafio será desenvolver uma aplicação utilizando os conceitos apresentados no curso como:

- Desenho de arquitetura
- Desenvolvimento de microsserviços
- Qualidade de Software
- Mensageria
- E outros conceitos abordados

## Requisitos Funcionais

Para ajudar o seu grupo nesta etapa de levantamento de requisitos, segue alguns dos pré-requisitos esperados para este projeto:

- A nova versão do sistema deve processar mais de um vídeo ao mesmo tempo
- Em caso de picos, o sistema não deve perder uma requisição
- O Sistema deve ser protegido por usuário e senha
- O fluxo deve ter uma listagem de status dos vídeos de um usuário
- Em caso de erro, um usuário pode ser notificado (e-mail ou outro meio de comunicação)

## Requisitos Técnicos

- O sistema deve persistir os dados
- O sistema deve estar em uma arquitetura que o permita ser escalado
- O projeto deve ser versionado no Github
- O projeto deve ter testes que garantam a sua qualidade
- CI/CD da aplicação

## Tecnologias

- Linguagem: Java
- Containers: Docker + Kubernetes
- Mensageria: RabbitMQ, Apache Kafka
- Banco de Dados: PostgreSQL + Redis (cache)
- Monitoramento: Prometheus + Grafana
- CI/CD: GitHub Actions

## Microserviços

- Autenticação: Um proxy de autenticação responsável por validar o usuário e senha de acesso ao sistema;
- Listagem de status: Um microsserviço responsável por listar os vídeo de um usuário;
- Processamento: Um microsserviço responsável por processar o vídeo e retornar as imagens em um arquivo .zip;
- Microserviço de notificação: Um microsserviço responsável por notificar o usuário por e-mail quando o processamento do vídeo for concluído.