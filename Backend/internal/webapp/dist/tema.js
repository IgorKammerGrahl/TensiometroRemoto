/* Tema antes da primeira pintura.
 *
 * A mesma regra de src/tema.ts, em JS puro: este arquivo roda antes do bundle
 * e nao importa nada. Arquivo externo, e nao <script> inline, porque a API
 * manda Content-Security-Policy com script-src 'self', que bloquearia script
 * inline. Sincrono e no <head>, depois das metas theme-color, para o tema
 * certo valer ja na primeira pintura. */
(function () {
  var escolha = null;
  try {
    escolha = localStorage.getItem('tema');
  } catch (e) {
    /* sem armazenamento: vale o padrao */
  }
  /* `only light` no claro: recusa o escurecimento automatico do navegador
   * (ver esquemaDe em src/tema.ts, com o que foi medido). */
  var esquema = escolha === 'escuro' ? 'dark' : escolha === 'sistema' ? 'light dark' : 'only light';
  document.documentElement.style.colorScheme = esquema;
  var metas = document.querySelectorAll('meta[name="theme-color"]');
  for (var i = 0; i < metas.length; i++) {
    var doEscuro = (metas[i].getAttribute('media') || '').indexOf('dark') >= 0;
    var escura = escolha === 'escuro' || (escolha === 'sistema' && doEscuro);
    metas[i].setAttribute('content', escura ? '#131210' : '#f7f5f0');
  }
})();
