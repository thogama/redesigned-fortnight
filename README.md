---
title: Crypto.com Card Spend Dashboard
emoji: 💳
colorFrom: blue
colorTo: indigo
sdk: docker
app_port: 7860
---

# Crypto.com Card Spend Dashboard

Dashboard em Go para importar o CSV exportado do cartão Crypto.com, classificar transações e consolidar gastos por mês.

## Classificação com OpenAI

Copie `.env.example` para `.env` e preencha `OPENAI_API_KEY`. A aplicação carrega o arquivo `.env` ao iniciar e usa a Responses API para classificar gastos não reconhecidos pelas regras locais. `OPENAI_MODEL` é opcional e usa `gpt-5.6-luna` por padrão.

Nunca versione a chave real. Sem a variável, ou se a API estiver indisponível, a classificação local continua funcionando normalmente.
