// Validacao dos campos do provisionamento e escape de HTML.
//
// C puro, sem nenhuma dependencia do Arduino, pelo mesmo motivo de
// payload.h: o codigo que decide o que pode ser gravado na NVS e o que
// pode ser escrito na pagina compila no host e e testado la
// (ver Circuito/teste/teste_prov.c).
//
// Este arquivo e a fronteira de confianca do portal. Dois dados chegam
// aqui vindos de fora: o SSID, que e anunciado por qualquer radio no
// alcance, e os campos digitados no formulario.

#ifndef PROV_VALIDA_H
#define PROV_VALIDA_H

#include <stddef.h>
#include <string.h>

// Token do dispositivo: base64url de 32 bytes. Ver cmd/devtoken no backend.
#define PROV_TOKEN_LEN     43
// Limite do servidor em http_app.go:278, nao um palpite desta tela.
#define PROV_DEVICE_ID_MAX 64

typedef enum {
  PROV_OK = 0,
  PROV_SSID_VAZIO,
  PROV_URL_VAZIA,
  PROV_URL_ESQUEMA,
  PROV_DEVICE_ID_VAZIO,
  PROV_DEVICE_ID_LONGO,
  PROV_TOKEN_TAMANHO,
  PROV_TOKEN_ALFABETO,
  PROV_TOKEN_EXIGIDO,
  PROV_CALIBRACAO_VAZIA
} ProvErro;

static int provVazio(const char *s) {
  return s == NULL || s[0] == '\0';
}

// provIgual trata NULL como string vazia: campo que nunca foi gravado na NVS
// e campo gravado vazio sao o mesmo "nao ha destino anterior".
static int provIgual(const char *a, const char *b) {
  if (a == NULL) {
    a = "";
  }
  if (b == NULL) {
    b = "";
  }
  return strcmp(a, b) == 0;
}

static int provBase64Url(char c) {
  return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
         (c >= '0' && c <= '9') || c == '-' || c == '_';
}

// Escapa para texto e para atributo entre aspas duplas. Devolve o numero
// de caracteres escritos (sem o NUL) ou -1 se nao coube.
//
// Nao couber e ERRO, nunca truncagem: um escape truncado pode terminar no
// meio de uma entidade ou de uma tag e produzir exatamente o HTML quebrado
// que esta funcao existe para impedir. Em caso de erro o destino sai vazio.
static int provEscaparHtml(char *destino, size_t tamanho, const char *origem) {
  if (destino == NULL || tamanho == 0) {
    return -1;
  }
  size_t escritos = 0;
  for (const char *p = origem; p != NULL && *p != '\0'; p++) {
    const char *entidade;
    switch (*p) {
      case '&':  entidade = "&amp;";  break;
      case '<':  entidade = "&lt;";   break;
      case '>':  entidade = "&gt;";   break;
      case '"':  entidade = "&quot;"; break;
      case '\'': entidade = "&#39;";  break;
      default:   entidade = NULL;     break;
    }
    if (entidade != NULL) {
      size_t n = strlen(entidade);
      if (escritos + n >= tamanho) {
        destino[0] = '\0';
        return -1;
      }
      memcpy(destino + escritos, entidade, n);
      escritos += n;
    } else {
      if (escritos + 1 >= tamanho) {
        destino[0] = '\0';
        return -1;
      }
      destino[escritos++] = *p;
    }
  }
  destino[escritos] = '\0';
  return (int) escritos;
}

// Valida os campos do formulario. permiteHttp reflete DEV_SEM_TLS.
//
// manterToken sai em 1 quando o campo do token veio vazio, que significa
// "mantenha o que ja esta na NVS" -- e o que permite trocar de Wi-Fi em
// campo sem redigitar 43 caracteres numa tela de celular.
//
// urlAtual e devAtual sao o DESTINO JA GRAVADO, e existem so para decidir se
// esse "manter" pode valer. Ver a justificativa no corpo.
static ProvErro provValidar(const char *ssid, const char *url,
                            const char *deviceId, const char *token,
                            const char *calibracao,
                            const char *urlAtual, const char *devAtual,
                            int permiteHttp, int *manterToken) {
  if (manterToken != NULL) {
    *manterToken = 0;
  }

  if (provVazio(ssid)) {
    return PROV_SSID_VAZIO;
  }
  if (provVazio(url)) {
    return PROV_URL_VAZIA;
  }

  // https sempre vale. http so com DEV_SEM_TLS compilado -- a decisao de
  // trafegar o token em claro e de quem compilou o firmware, nunca de quem
  // preenche o formulario.
  int ehHttps = strncmp(url, "https://", 8) == 0;
  int ehHttp = strncmp(url, "http://", 7) == 0;
  if (!ehHttps && !(ehHttp && permiteHttp)) {
    return PROV_URL_ESQUEMA;
  }

  if (provVazio(deviceId)) {
    return PROV_DEVICE_ID_VAZIO;
  }
  if (strlen(deviceId) > PROV_DEVICE_ID_MAX) {
    return PROV_DEVICE_ID_LONGO;
  }

  if (provVazio(token)) {
    // MANTER O TOKEN SO VALE ENQUANTO O DESTINO NAO MUDA.
    //
    // "Deixe em branco para manter o token gravado" existe para uma coisa:
    // trocar de Wi-Fi no talhao sem redigitar 43 caracteres. Junto de um
    // campo de URL livre, vira outra: quem alcancar o portal aponta o no
    // para o proprio servidor, deixa o token em branco, e o no reinicia
    // entregando o token REAL no header Authorization. O segredo sai sem
    // nunca ter sido exibido na tela, e o dono nao tem como perceber.
    //
    // Por isso a regra e sobre o DESTINO, e nao sobre quem esta na tela: url
    // e device_id sao para onde, e em nome de quem, o token vai ser
    // apresentado. Mudar qualquer um dos dois passa a exigir o token
    // digitado por inteiro -- ou seja, exige ja te-lo. Trocar SSID, senha do
    // Wi-Fi ou calibracao nao exige nada disso: nenhum deles muda a quem o
    // segredo e entregue.
    //
    // A comparacao e literal, sem normalizar. "https://A.B/x" e
    // "https://a.b/x" sao o mesmo lugar e ainda assim vao pedir o token:
    // falhar fechado custa uma redigitacao, e normalizar URL na mao custa
    // uma brecha que ninguem ve. No virgem cai aqui tambem, porque urlAtual
    // vem vazia -- e esta certo, um no sem token gravado nao tem o que
    // manter.
    if (!provIgual(url, urlAtual) || !provIgual(deviceId, devAtual)) {
      return PROV_TOKEN_EXIGIDO;
    }
    if (manterToken != NULL) {
      *manterToken = 1;
    }
  } else {
    // Comprimento e alfabeto pegam o caso mais provavel em campo: colagem
    // truncada. Nao provam que o token e valido -- so o servidor prova --
    // mas impedem gravar um valor que com certeza nao autentica.
    if (strlen(token) != PROV_TOKEN_LEN) {
      return PROV_TOKEN_TAMANHO;
    }
    for (const char *p = token; *p != '\0'; p++) {
      if (!provBase64Url(*p)) {
        return PROV_TOKEN_ALFABETO;
      }
    }
  }

  if (provVazio(calibracao)) {
    return PROV_CALIBRACAO_VAZIA;
  }
  return PROV_OK;
}

static const char *provErroTexto(ProvErro e) {
  switch (e) {
    case PROV_OK:                return "ok";
    case PROV_SSID_VAZIO:        return "Escolha uma rede Wi-Fi.";
    case PROV_URL_VAZIA:         return "Informe a URL da API.";
    // A mensagem acompanha o que o build consegue falar, pelo mesmo motivo
    // do placeholder do formulario: num build DEV_SEM_TLS mandar usar https
    // manda usar o que o firmware nao atende.
#ifdef DEV_SEM_TLS
    case PROV_URL_ESQUEMA:       return "A URL precisa comecar com http:// ou https://";
#else
    case PROV_URL_ESQUEMA:       return "A URL precisa comecar com https://";
#endif
    case PROV_DEVICE_ID_VAZIO:   return "Informe o identificador do no.";
    case PROV_DEVICE_ID_LONGO:   return "O identificador passa de 64 caracteres.";
    case PROV_TOKEN_TAMANHO:     return "O token precisa ter 43 caracteres. Confira se a colagem veio inteira.";
    case PROV_TOKEN_ALFABETO:    return "O token tem caractere invalido. Confira a colagem.";
    case PROV_TOKEN_EXIGIDO:     return "Mudar a URL ou o identificador do no exige colar o token de novo.";
    case PROV_CALIBRACAO_VAZIA:  return "Informe a calibracao.";
  }
  return "Configuracao invalida.";
}

// O seq e persistido na NVS e o servidor e idempotente por
// (device_id, seq). Reaproveitar o hardware com outro device_id mantendo o
// seq faria um no novo comecar numa sequencia alta; e se ele voltasse ao id
// antigo depois, produziria duplicatas silenciosas. Trocar o id zera o seq.
static int provDeveZerarSeq(const char *antigo, const char *novo) {
  if (provVazio(antigo) || provVazio(novo)) {
    return 0;
  }
  return strcmp(antigo, novo) != 0;
}

#endif  // PROV_VALIDA_H
