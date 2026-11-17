import { useSyncExternalStore } from 'react';

// Instalar o aplicativo por um botao em Ajustes (guia install-web-app).
//
// Dois caminhos, nesta ordem: navigator.install() (Web Install API, quando o
// navegador tiver) e, sem ele, o evento beforeinstallprompt do Chrome, que
// chega cedo -- antes de qualquer tela montar -- e so uma vez. Por isso a
// captura comeca em main.tsx e o evento fica guardado aqui ate o toque.
//
// Sem nenhum dos dois, o botao NAO aparece: botao que nao faz nada e pior que
// nenhum. Ajustes mostra entao a instrucao em texto, conferida no Chrome do
// Android (menu ⋮ -> "Adicionar a tela inicial").

type PedidoDeInstalacao = Event & {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
};

type ComInstall = Navigator & { install?: () => Promise<void> };

export type Resultado = 'instalado' | 'cancelado' | 'falhou';

let pendente: PedidoDeInstalacao | null = null;
const ouvintes = new Set<() => void>();
const avisar = () => ouvintes.forEach((f) => f());

const temWebInstall = () => typeof (navigator as ComInstall).install === 'function';

export function capturarPedidoDeInstalacao(): void {
  window.addEventListener('beforeinstallprompt', (e) => {
    // Sem isto o Chrome mostraria o proprio aviso na hora que quisesse; aqui
    // ele espera a pessoa pedir.
    e.preventDefault();
    pendente = e as PedidoDeInstalacao;
    avisar();
  });
  window.addEventListener('appinstalled', () => {
    pendente = null;
    avisar();
  });
}

const inscrever = (f: () => void) => {
  ouvintes.add(f);
  return () => {
    ouvintes.delete(f);
  };
};

/** A funcao que instala, ou null quando nao ha como pelo aplicativo. Chamar
 *  DENTRO do clique: os dois caminhos exigem o gesto da pessoa. */
export function usePedidoDeInstalacao(): (() => Promise<Resultado>) | null {
  const p = useSyncExternalStore(inscrever, () => pendente);
  if (temWebInstall()) {
    return async () => {
      try {
        await (navigator as ComInstall).install!();
        return 'instalado';
      } catch (erro) {
        return erro instanceof DOMException && erro.name === 'AbortError' ? 'cancelado' : 'falhou';
      }
    };
  }
  if (!p) return null;
  return async () => {
    // O evento serve uma vez so: sai daqui antes de esperar a resposta.
    pendente = null;
    avisar();
    try {
      await p.prompt();
      const { outcome } = await p.userChoice;
      return outcome === 'accepted' ? 'instalado' : 'cancelado';
    } catch {
      return 'falhou';
    }
  };
}
