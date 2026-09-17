/// <reference types="vitest/config" />
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

// Uma linha em vez de @types/node inteiro: e a unica coisa que este arquivo
// usa do Node, e o pacote de tipos so serviria para ela.
declare const process: { env: Record<string, string | undefined> };

export default defineConfig({
  plugins: [react()],

  server: {
    // host: true expoe o dev server na rede local, para abrir a PWA no
    // celular apontando para http://192.168.x.x:5173. Combina com
    // SESSAO_COOKIE_INSEGURO=true no backend: origem HTTP de rede local nao
    // e trustworthy para o navegador, e o cookie Secure seria descartado em
    // silencio -- o login pareceria simplesmente nao funcionar.
    host: true,

    // O PROXY NAO E CONVENIENCIA, E O QUE FAZ A SESSAO FUNCIONAR EM DEV.
    //
    // Em producao o binario Go serve a PWA e a API na mesma origem. Em
    // desenvolvimento sao duas portas -- 5173 e 8080 -- portanto duas
    // origens, e o cookie SameSite=Lax nao acompanha a requisicao. Sem este
    // proxy a autenticacao falha sem nenhum sintoma que aponte a causa: o
    // login responde 204, o cookie e descartado calado, e a proxima
    // requisicao volta 401.
    //
    // Com o proxy o navegador ve uma origem so nos dois ambientes.
    proxy: { '/api': process.env['API'] ?? 'http://localhost:8080' },
  },

  build: {
    // O build sai direto dentro do modulo Go que o embute. embed nao aceita
    // caminho com "..", entao o destino precisa morar sob o modulo -- e nao
    // ha etapa de copia entre `npm run build` e `go build`.
    outDir: '../Backend/internal/webapp/dist',
    emptyOutDir: true,
  },

  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],

    // FUSO FIXO EM UTC-3, E NAO O DA MAQUINA.
    //
    // Sem isto a suite passa ou nao conforme o relogio de quem a roda, e o
    // caso que some e justamente o que interessa: uma data DATE ("2026-09-17")
    // que passe por `new Date` vira meia-noite UTC e recua um dia a oeste de
    // Greenwich -- em UTC a conversao errada da o mesmo resultado que a certa,
    // e o defeito so aparece no celular de quem esta em campo.
    //
    // America/Sao_Paulo porque e onde o sistema roda (IFC Rio do Sul), e
    // porque nao tem mais horario de verao desde 2019: o deslocamento e -3
    // o ano inteiro, entao o fuso nao introduz sazonalidade nos testes.
    env: { TZ: 'America/Sao_Paulo' },
  },
});
