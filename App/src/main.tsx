import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import './estilo/base.css';

const raiz = document.getElementById('raiz');
if (!raiz) throw new Error('elemento #raiz ausente em index.html');

createRoot(raiz).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

/* Service worker so em producao.
 *
 * Em desenvolvimento ele fica entre o navegador e o dev server e transforma
 * qualquer confusao de cache num falso bug de aplicacao -- e o proprio worker
 * nao guarda nada (ver public/sw.js), entao nao ha o que testar em dev. */
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {
      // Instalacao indisponivel nao e motivo para derrubar o aplicativo: o PWA
      // continua funcionando como pagina.
    });
  });
}
