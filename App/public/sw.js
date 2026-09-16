/* Service worker deliberadamente sem cache.
 *
 * ELE EXISTE PARA UMA COISA SO: ser o service worker que os navegadores exigem
 * para oferecer "Instalar aplicativo". Instalacao esta no escopo; funcionamento
 * offline NAO esta.
 *
 * A tentacao obvia seria guardar as ultimas respostas da API para a tela abrir
 * com alguma coisa quando nao ha rede. Isso destruiria o requisito central da
 * interface: distinguir leitura atual de leitura velha. Um cache serviria um
 * valor de tres horas atras com a mesma cara de um valor de agora -- e o
 * aplicativo inteiro foi desenhado para que isso nunca aconteca. Sem rede, a
 * resposta honesta e dizer que nao ha rede.
 *
 * Guardar so o shell (HTML, JS, CSS) e defensavel e continua fora de escopo: o
 * shell sem API abre numa tela que so sabe dizer "sem conexao", que e o que o
 * proprio navegador ja diz.
 *
 * ponytail: o handler de fetch e passa-adiante de proposito. Se um dia houver
 * escopo de offline, o lugar da mudanca e aqui -- e a primeira regra tera de ser
 * "nunca /api/".
 */

self.addEventListener('install', () => {
  // Sem cache para preencher, entao nada a esperar.
  self.skipWaiting();
});

self.addEventListener('activate', (evento) => {
  evento.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', () => {
  // Sem respondWith: o navegador segue direto para a rede, como se o service
  // worker nao existisse. O handler precisa existir; o cache, nao.
});
