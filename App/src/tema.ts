// Tema: CLARO POR PADRAO, e nao o do sistema.
//
// Desvio deliberado do guia dark-mode-toggle (modern-web-guidance), que manda
// seguir o sistema. Dois motivos de campo: o aplicativo e lido no sol, onde o
// claro e o legivel; e a economia de bateria do Android liga o tema escuro
// sozinha -- o produtor com pouca bateria receberia a tela escura sem ter
// escolhido nada. Quem quer o escuro escolhe em Ajustes.
//
// A MESMA REGRA EXISTE EM public/tema.js, em JS puro, porque aquele arquivo
// roda antes do bundle (para nao piscar o tema errado) e nao pode importar
// nada. Mudou aqui, muda la.

export type Escolha = 'claro' | 'escuro' | 'sistema';
export type Esquema = 'only light' | 'dark' | 'light dark';

export const CHAVE_TEMA = 'tema';

const COR_CLARA = '#f7f5f0';
const COR_ESCURA = '#131210';

/** Valor guardado -> escolha. Qualquer coisa que nao seja uma das duas
 *  escolhas explicitas vira claro: o lado seguro e o padrao. */
export function escolhaDe(valor: string | null): Escolha {
  return valor === 'escuro' || valor === 'sistema' ? valor : 'claro';
}

/** `only light`, e nao `light`, no claro: `only` e a forma padrao de recusar o
 *  escurecimento automatico do navegador, que inverteria por algoritmo as
 *  cores de estado e as hachuras validadas para daltonismo. Quem quer o escuro
 *  tem o nosso, escolhido em Ajustes.
 *
 *  O QUE FOI MEDIDO (emulador, Chrome 149, 17/11/2026):
 *  - tema escuro do proprio Chrome, o caso comum: o app continua claro
 *    (prefers-color-scheme: dark e verdadeiro, e nao muda nada aqui);
 *  - "tema escuro para sites" NAO existe mais nas configuracoes; so a flag
 *    chrome://flags "Auto Dark Mode for Web Contents", e ela IGNORA o `only`:
 *    light-dark() resolve para o escuro (aparece a nossa paleta escura) e
 *    algumas bordas saem invertidas. Caso raro e fora do nosso alcance. */
export function esquemaDe(e: Escolha): Esquema {
  return e === 'escuro' ? 'dark' : e === 'sistema' ? 'light dark' : 'only light';
}

/** Cor da barra do navegador para uma meta theme-color. `metaDoEscuro` diz
 *  se e a meta com media="(prefers-color-scheme: dark)". */
export function corDaBarra(e: Escolha, metaDoEscuro: boolean): string {
  if (e === 'escuro') return COR_ESCURA;
  if (e === 'sistema' && metaDoEscuro) return COR_ESCURA;
  return COR_CLARA;
}

/** localStorage pode lancar (dados do site bloqueados, aba anonima). Sem
 *  armazenamento, vale o padrao. */
export function lerEscolha(): Escolha {
  try {
    return escolhaDe(localStorage.getItem(CHAVE_TEMA));
  } catch {
    return 'claro';
  }
}

/** Claro e o padrao, entao escolher claro APAGA a chave em vez de grava-la. */
export function gravarEscolha(e: Escolha): void {
  try {
    if (e === 'claro') localStorage.removeItem(CHAVE_TEMA);
    else localStorage.setItem(CHAVE_TEMA, e);
  } catch {
    // Sem armazenamento a escolha vale so ate fechar a aba. Nao ha o que avisar.
  }
}

/** color-scheme inline no <html>: vence o `color-scheme: light` do :root, e o
 *  light-dark() dos tokens segue o resultado. */
export function aplicarEscolha(e: Escolha): void {
  document.documentElement.style.colorScheme = esquemaDe(e);
  for (const m of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    m.content = corDaBarra(e, m.media.includes('dark'));
  }
}

/** Outra aba mudou o tema: acompanha. */
export function escutarOutrasAbas(): void {
  window.addEventListener('storage', (ev) => {
    if (ev.key === CHAVE_TEMA || ev.key === null) aplicarEscolha(lerEscolha());
  });
}
