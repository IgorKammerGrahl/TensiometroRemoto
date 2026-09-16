// Portal cativo de configuracao do no.
//
// SOBE APENAS EM DOIS CASOS: NVS sem configuracao completa, ou botao BOOT
// pressionado no reset. Nunca coexiste com a telemetria -- salvar reinicia
// o dispositivo. Consequencia boa de segunda ordem: WiFiClientSecure
// (mbedTLS, dezenas de KB de heap por handshake) e WebServer jamais
// dividem a RAM.
//
// NAO EXISTE FALLBACK AUTOMATICO para este modo quando o Wi-Fi cai. Seria
// tentador e e errado: quem consegue derrubar o Wi-Fi passaria a conseguir
// levantar o AP de configuracao.
//
// O SSID exibido vem do scan, ou seja, de qualquer radio no alcance. Ele e
// ESCAPADO antes de entrar no HTML (provEscaparHtml). Uma rede vizinha
// chamada <script>... e entrada de terceiro renderizada na tela de quem
// configura.
//
// QUEM ALCANCA ESTE PORTAL CONSEGUE RECONFIGURAR O NO. A senha do AP
// (PROV_AP_SENHA) e um valor compartilhado que precisa ser conhecido por
// quem instala, e o valor do config.h.example e publico. Nao ha segredo
// possivel ai: a placa vem gravada, o instalador so tem um celular, e nao
// existe canal fora de banda para combinar uma senha por unidade. Derivar do
// MAC seria teatro -- o MAC vai no beacon, qualquer um no alcance le.
//
// O que entao limita o estrago, em camadas:
//
//   1. o AP so existe com NVS incompleta ou apos BOOT fisico, e cai em 10min
//   2. o token gravado NUNCA e reemitido para o HTML
//   3. manter o token em branco so vale se url e device_id nao mudarem
//      (provValidar) -- sem isso, apontar o no para outro servidor exfiltra
//      o token real no primeiro POST
//
// Sobra, declarado: quem alcancar o portal pode gravar uma configuracao
// ERRADA e deixar o no mudo ate alguem reprovisionar. E negacao de servico
// com presenca fisica, nao roubo de credencial. Quem for operar isso a serio
// troca PROV_AP_SENHA no proprio config.h, que nao e versionado.

#ifndef PROV_H
#define PROV_H

#include <Arduino.h>
#include <DNSServer.h>
#include <WebServer.h>
#include <WiFi.h>
#include <esp_mac.h>

#include "prov_nvs.h"
#include "prov_valida.h"

#define PROV_TIMEOUT_MS   600000UL   // 10 min sem salvar derrubam o AP
#define PROV_TESTE_MS     15000UL    // espera maxima da associacao de teste

static WebServer provServidor(80);
static DNSServer provDns;

// Resultado do teste de associacao. Vive em RAM porque o celular cai
// durante o teste (ver "troca de canal", abaixo) e precisa reencontrar o
// veredito quando voltar.
enum ProvTeste { PROV_TESTE_OCIOSO, PROV_TESTE_RODANDO,
                 PROV_TESTE_OK, PROV_TESTE_FALHOU };
static ProvTeste provTesteEstado = PROV_TESTE_OCIOSO;
static String provTesteSsid;
static String provTesteSenha;
static bool provTestePendente = false;

static ProvConfig provAtual;

static String provEscapar(const String &s) {
  char buf[256];
  if (provEscaparHtml(buf, sizeof buf, s.c_str()) < 0) {
    return String("(nome muito longo)");
  }
  return String(buf);
}

// esp_read_mac le o MAC direto do eFuse. WiFi.macAddress() depende da
// interface ja estar levantada e devolve o buffer zerado se chamado antes
// -- o AP saia como "tensio-setup-0000", igual em toda placa.
static String provNomeAp() {
  uint8_t mac[6] = {0};
  esp_read_mac(mac, ESP_MAC_WIFI_STA);
  char nome[32];
  snprintf(nome, sizeof nome, "tensio-setup-%02X%02X", mac[4], mac[5]);
  return String(nome);
}

// O token NUNCA e reemitido para o HTML. Campo em branco, com aviso de que
// deixar vazio mantem o atual. Ecoa-lo poria o segredo numa pagina servida
// por um AP compartilhado, e no historico do navegador.
static String provPagina() {
  String h = F("<!doctype html><html lang=\"pt-br\"><meta charset=\"utf-8\">"
               "<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"
               "<title>Configurar no</title><style>"
               "body{font-family:system-ui,sans-serif;margin:0;padding:1.5rem;max-width:32rem}"
               "label{display:block;margin:.9rem 0 .25rem;font-weight:600}"
               "input,select{width:100%;padding:.6rem;font-size:1rem;box-sizing:border-box}"
               "button{margin-top:1rem;width:100%;padding:.8rem;font-size:1rem}"
               ".dica{font-size:.85rem;color:#555;margin:.25rem 0 0}"
               "#res{margin-top:1rem;padding:.6rem;border-left:4px solid #999}"
               "</style><h1>Configurar nó</h1>"
               "<form method=\"POST\" action=\"/salvar\">"
               "<label for=\"ssid\">Rede Wi-Fi</label><select id=\"ssid\" name=\"ssid\">");

  // Le o scan RETIDO, feito uma vez na subida do portal. Varrer aqui
  // derrubaria o proprio cliente que esta carregando esta pagina: o scan
  // e bloqueante e passeia pelos canais, e o AP vai junto. Por isso nao
  // ha scanDelete() -- os resultados ficam retidos de proposito, e so a
  // rota /rescan os substitui.
  int n = WiFi.scanComplete();
  for (int i = 0; i < n; i++) {
    String nome = provEscapar(WiFi.SSID(i));
    h += "<option value=\"" + nome + "\"";
    if (WiFi.SSID(i) == provAtual.ssid) {
      h += " selected";
    }
    h += ">" + nome + " (" + String(WiFi.RSSI(i)) + " dBm)</option>";
  }
  if (n <= 0) {
    h += F("<option value=\"\">Nenhuma rede encontrada</option>");
  }

  h += F("</select>"
         "<label for=\"senha\">Senha da rede</label>"
         "<input id=\"senha\" name=\"senha\" type=\"password\" autocomplete=\"off\">"
         "<p class=\"dica\"><a href=\"/rescan\">Procurar redes de novo</a> "
         "— a conexão com o nó pisca durante a busca.</p>"
         "<button type=\"button\" onclick=\"testar()\">Testar esta rede</button>"
         "<p class=\"dica\">A conexão com o nó pode piscar durante o teste. "
         "É esperado: o rádio é um só. Se o celular sair, reconecte e o "
         "resultado aparece aqui.</p><div id=\"res\">Nenhum teste feito.</div>"
         "<label for=\"url\">URL da API</label>"
         "<input id=\"url\" name=\"url\" type=\"url\" value=\"");
  h += provEscapar(provAtual.url);
  // O placeholder acompanha o que o build consegue falar. Num build
  // DEV_SEM_TLS o cliente e WiFiClient puro: sugerir https:// aqui empurra
  // quem configura para uma URL que o firmware nao tem como atender.
#ifdef DEV_SEM_TLS
  h += F("\" placeholder=\"http://192.168.0.10:8080/api/v1/readings\">");
#else
  h += F("\" placeholder=\"https://.../api/v1/readings\">");
#endif
  h += F("<label for=\"dev\">Identificador do nó</label>"
         "<input id=\"dev\" name=\"dev\" maxlength=\"64\" autocapitalize=\"none\" "
         "autocorrect=\"off\" spellcheck=\"false\" value=\"");
  h += provEscapar(provAtual.deviceId);
  h += F("\">"
         "<label for=\"tok\">Token do nó</label>"
         "<input id=\"tok\" name=\"tok\" autocapitalize=\"none\" autocorrect=\"off\" "
         "spellcheck=\"false\" placeholder=\"43 caracteres\">"
         "<p class=\"dica\">");
  h += provAtual.token.length() > 0
           ? F("Deixe em branco para manter o token que já está gravado.")
           : F("Cole o token que o aplicativo mostrou uma única vez.");
  h += F("</p>"
         "<label for=\"cal\">Calibração</label>"
         "<input id=\"cal\" name=\"cal\" autocapitalize=\"none\" spellcheck=\"false\" value=\"");
  h += provEscapar(provAtual.calibracao);
  h += F("\"><button type=\"submit\">Salvar e reiniciar</button></form>"
         "<script>"
         "function testar(){var d=document.getElementById('res');"
         "d.textContent='Testando... o celular pode cair e voltar.';"
         "var b=new URLSearchParams({ssid:document.getElementById('ssid').value,"
         "senha:document.getElementById('senha').value});"
         "fetch('/testar',{method:'POST',body:b}).catch(function(){});"
         "setTimeout(sondar,3000);}"
         "function sondar(){fetch('/testar/resultado').then(function(r){return r.text()})"
         ".then(function(t){var d=document.getElementById('res');d.textContent=t;"
         "if(t.indexOf('Testando')===0){setTimeout(sondar,2000);}})"
         ".catch(function(){setTimeout(sondar,2000);});}"
         "</script>");
  return h;
}

static void provHandleRaiz() {
  provServidor.send(200, "text/html; charset=utf-8", provPagina());
}

// Responde ANTES de associar. A associacao roda no laco do portal.
//
// A TROCA DE CANAL: o radio e um so. Em WIFI_AP_STA o SoftAP e obrigado a
// operar no canal da conexao STA, entao no instante em que o teste associa
// numa rede de canal diferente, o ESP32 muda o canal do proprio AP e
// derruba o celular conectado a ele. Se a resposta fosse enviada depois da
// associacao, ela nunca chegaria, e o sintoma -- "o portal travou" -- seria
// a leitura oposta do que esta acontecendo.
// Varrer e bloqueante e passeia pelos canais, entao derruba o celular do
// AP. Responder o 302 ANTES de varrer faz o navegador ja ter o destino na
// mao quando a conexao voltar.
static void provHandleRescan() {
  provServidor.sendHeader("Location", "/");
  provServidor.send(302, "text/plain", "");
  delay(100);
  WiFi.scanNetworks();
}

static void provHandleTestar() {
  provTesteSsid = provServidor.arg("ssid");
  provTesteSenha = provServidor.arg("senha");
  if (provTesteSsid.length() == 0) {
    provServidor.send(400, "text/plain; charset=utf-8", "Escolha uma rede.");
    return;
  }
  provTesteEstado = PROV_TESTE_RODANDO;
  provTestePendente = true;
  provServidor.send(202, "text/plain; charset=utf-8", "Testando...");
}

static void provHandleResultado() {
  const char *texto;
  switch (provTesteEstado) {
    case PROV_TESTE_RODANDO: texto = "Testando... o celular pode cair e voltar."; break;
    case PROV_TESTE_OK:      texto = "Conectou nesta rede."; break;
    case PROV_TESTE_FALHOU:  texto = "Nao conectou. Confira a senha e o alcance."; break;
    default:                 texto = "Nenhum teste feito."; break;
  }
  provServidor.send(200, "text/plain; charset=utf-8", texto);
}

static void provHandleSalvar() {
  String ssid = provServidor.arg("ssid");
  String senha = provServidor.arg("senha");
  String url = provServidor.arg("url");
  String dev = provServidor.arg("dev");
  String tok = provServidor.arg("tok");
  String cal = provServidor.arg("cal");

#ifdef DEV_SEM_TLS
  const int permiteHttp = 1;
#else
  const int permiteHttp = 0;
#endif

  int manterToken = 0;
  // provAtual e o destino JA GRAVADO. Ele entra na validacao porque e o que
  // decide se o token pode ser mantido em branco -- ver provValidar.
  ProvErro e = provValidar(ssid.c_str(), url.c_str(), dev.c_str(),
                           tok.c_str(), cal.c_str(),
                           provAtual.url.c_str(), provAtual.deviceId.c_str(),
                           permiteHttp, &manterToken);
  if (e != PROV_OK) {
    String msg = String(F("<!doctype html><meta charset=\"utf-8\"><h1>Não salvou</h1><p>")) +
                 provEscapar(String(provErroTexto(e))) +
                 F("</p><p><a href=\"/\">Voltar</a></p>");
    provServidor.send(400, "text/html; charset=utf-8", msg);
    return;
  }

  // Token em branco mantem o que ja esta gravado. Se nao ha nada gravado,
  // manter significaria gravar vazio -- e um no sem token nunca autentica.
  if (manterToken) {
    if (provAtual.token.length() == 0) {
      provServidor.send(400, "text/html; charset=utf-8",
                        F("<!doctype html><meta charset=\"utf-8\"><h1>Não salvou</h1>"
                          "<p>Este nó ainda não tem token gravado, então o campo "
                          "não pode ficar em branco.</p><p><a href=\"/\">Voltar</a></p>"));
      return;
    }
    tok = provAtual.token;
  }

  ProvConfig nova;
  nova.ssid = ssid;
  nova.senha = senha;
  nova.url = url;
  nova.deviceId = dev;
  nova.token = tok;
  nova.calibracao = cal;

  if (!provGravar(nova)) {
    provServidor.send(500, "text/html; charset=utf-8",
                      F("<!doctype html><meta charset=\"utf-8\"><h1>Falha ao gravar</h1>"
                        "<p>A NVS recusou a gravação.</p><p><a href=\"/\">Voltar</a></p>"));
    return;
  }

  provServidor.send(200, "text/html; charset=utf-8",
                    F("<!doctype html><meta charset=\"utf-8\"><h1>Salvo</h1>"
                      "<p>O nó está reiniciando em modo telemetria. "
                      "Esta rede de configuração vai sumir.</p>"));
  delay(1500);
  ESP.restart();
}

// Portal cativo: qualquer host resolve para o IP do AP, e qualquer rota
// desconhecida redireciona para a raiz. E isso que faz Android e iOS
// abrirem a pagina sozinhos.
static void provHandleNaoEncontrado() {
  provServidor.sendHeader("Location",
                          String("http://") + WiFi.softAPIP().toString() + "/");
  provServidor.send(302, "text/plain", "");
}

static void provExecutarTeste() {
  Serial.print("# Portal: testando associacao em ");
  Serial.println(provTesteSsid);

  WiFi.begin(provTesteSsid.c_str(), provTesteSenha.c_str());
  unsigned long inicio = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - inicio < PROV_TESTE_MS) {
    delay(100);
  }
  bool ok = WiFi.status() == WL_CONNECTED;
  provTesteEstado = ok ? PROV_TESTE_OK : PROV_TESTE_FALHOU;

  Serial.println(ok ? "# Portal: associou." : "# Portal: NAO associou.");

  // Desconecta para devolver o AP ao canal proprio e reencontrar o celular.
  WiFi.disconnect(false, false);
  delay(200);
}

static void provRodarPortal(const ProvConfig &atual) {
  provAtual = atual;

  Serial.println("# MODO PROVISIONAMENTO");

  WiFi.mode(WIFI_AP_STA);
  String nome = provNomeAp();
  WiFi.softAP(nome.c_str(), PROV_AP_SENHA);
  delay(300);

  Serial.print("# AP: ");
  Serial.print(nome);
  Serial.print("   IP: ");
  Serial.println(WiFi.softAPIP());

  // Varredura inicial, ANTES de qualquer cliente associar: e o unico
  // momento em que ela nao custa a conexao de ninguem. Os resultados ficam
  // retidos (sem scanDelete) e alimentam a pagina.
  Serial.println("# Portal: varrendo redes...");
  int redes = WiFi.scanNetworks();
  Serial.print("# Portal: ");
  Serial.print(redes);
  Serial.println(" redes encontradas.");

  provDns.setTTL(0);
  provDns.start(53, "*", WiFi.softAPIP());

  provServidor.on("/", HTTP_GET, provHandleRaiz);
  provServidor.on("/rescan", HTTP_GET, provHandleRescan);
  provServidor.on("/testar", HTTP_POST, provHandleTestar);
  provServidor.on("/testar/resultado", HTTP_GET, provHandleResultado);
  provServidor.on("/salvar", HTTP_POST, provHandleSalvar);
  provServidor.onNotFound(provHandleNaoEncontrado);
  provServidor.begin();

  unsigned long inicio = millis();
  while (millis() - inicio < PROV_TIMEOUT_MS) {
    provDns.processNextRequest();
    provServidor.handleClient();
    if (provTestePendente) {
      provTestePendente = false;
      provExecutarTeste();
    }
    delay(2);
  }

  // Timeout. O AP nao sobrevive a saida do operador.
  Serial.println("# Portal: 10 min sem salvar. Desligando o AP.");
  provServidor.stop();
  provDns.stop();
  WiFi.softAPdisconnect(true);
  WiFi.mode(WIFI_OFF);
}

// Nao retorna. Nao volta sozinho ao portal e nao tenta telemetria com
// configuracao incompleta -- as duas coisas seriam formas de o AP
// reaparecer, ou de o no falhar em laco, sem ninguem por perto.
static void provOcioso() {
  Serial.println("# No sem configuracao e sem portal.");
  Serial.println("# Para reabrir o portal: pressione RESET e, DEPOIS de");
  Serial.println("# SOLTAR o RESET, segure BOOT por cerca de 3 segundos.");
  Serial.println("# O gesto e sequencial. BOOT ja segurado no instante em");
  Serial.println("# que o RESET e solto poe a placa em modo de gravacao,");
  Serial.println("# e o firmware nem chega a rodar.");
  while (true) {
    delay(1000);
  }
}

#endif  // PROV_H
