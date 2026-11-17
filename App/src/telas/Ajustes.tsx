import { useEffect, useState } from 'react';
import { Link } from 'react-router';
import { type Resultado, usePedidoDeInstalacao } from '../instalacao';
import { useLogout, useSessao } from '../sessao';
import { aplicarEscolha, type Escolha, gravarEscolha, lerEscolha } from '../tema';

// Ajustes: o que a pessoa configura UMA vez, longe da pergunta da tela
// inicial. Aparencia com tres estados explicitos (guia dark-mode-toggle: numa
// tela de ajustes, tres estados ficam claros), instalacao, cadastro e conta.

const OPCOES: ReadonlyArray<{ valor: Escolha; rotulo: string; dica: string }> = [
  { valor: 'claro', rotulo: 'Claro', dica: 'Melhor para ler no sol. É o padrão.' },
  { valor: 'escuro', rotulo: 'Escuro', dica: 'Mais confortável à noite.' },
  { valor: 'sistema', rotulo: 'Igual ao celular', dica: 'Segue o modo claro ou escuro do aparelho.' },
];

const ESCURO = '(prefers-color-scheme: dark)';

/** O modo do aparelho agora, para "Igual ao celular (agora: escuro)". */
function useSistemaEscuro(): boolean {
  const [escuro, setEscuro] = useState(() => matchMedia(ESCURO).matches);
  useEffect(() => {
    const m = matchMedia(ESCURO);
    const ouvir = () => setEscuro(m.matches);
    m.addEventListener('change', ouvir);
    return () => m.removeEventListener('change', ouvir);
  }, []);
  return escuro;
}

export function Ajustes() {
  const sessao = useSessao();
  const logout = useLogout();
  const [escolha, setEscolha] = useState<Escolha>(lerEscolha);
  const sistemaEscuro = useSistemaEscuro();
  // Instalado, o aplicativo ja abre pelo icone: a instrucao so ocuparia lugar.
  const instalado = matchMedia('(display-mode: standalone)').matches;
  const instalar = usePedidoDeInstalacao();
  const [instalando, setInstalando] = useState(false);
  const [resultado, setResultado] = useState<Resultado | null>(null);

  const escolher = (e: Escolha) => {
    setEscolha(e);
    gravarEscolha(e);
    aplicarEscolha(e);
  };

  return (
    <div className="pagina">
      <header className="cabecalho cabecalho--detalhe">
        <Link to="/" className="voltar">
          ← Talhões
        </Link>
      </header>
      <h1 className="ajustes__titulo">Ajustes</h1>

      <div className="pilha ajustes">
        <fieldset className="opcoes">
          <legend>Aparência</legend>
          {OPCOES.map((o) => (
            <label key={o.valor} className="opcao">
              <input
                type="radio"
                name="tema"
                value={o.valor}
                checked={escolha === o.valor}
                onChange={() => escolher(o.valor)}
              />
              <span className="opcao__texto">
                <span className="opcao__rotulo">
                  {o.rotulo}
                  {o.valor === 'sistema' && ` (agora: ${sistemaEscuro ? 'escuro' : 'claro'})`}
                </span>
                <span className="opcao__dica">{o.dica}</span>
              </span>
            </label>
          ))}
        </fieldset>

        {!instalado && (
          <section className="pilha conteudo">
            <h2>Instalar o aplicativo</h2>
            {instalar ? (
              <button
                type="button"
                className="botao botao--largo"
                /* aria-disabled, e nao disabled: o botao nao perde o foco
                   enquanto o pedido esta aberto (guia install-web-app). */
                aria-disabled={instalando}
                onClick={() => {
                  if (instalando) return;
                  setInstalando(true);
                  void instalar()
                    .then(setResultado)
                    .finally(() => setInstalando(false));
                }}
              >
                Instalar o aplicativo
              </button>
            ) : (
              <p>
                No menu <strong>⋮</strong> do navegador, toque em <strong>Adicionar à tela inicial</strong>.
              </p>
            )}
            <p className="secundario">O Tensio passa a abrir como um aplicativo, pelo ícone.</p>
            <p role="status" className="secundario">
              {resultado === 'instalado' && 'Pronto: o aplicativo foi instalado ou aberto.'}
              {resultado === 'cancelado' && 'A instalação foi cancelada. Dá para tentar de novo.'}
              {resultado === 'falhou' && 'Não foi possível instalar agora.'}
            </p>
          </section>
        )}

        <section className="pilha conteudo">
          <h2>Sensores</h2>
          <Link to="/nos/novo" className="botao botao--contorno botao--largo">
            Cadastrar sensor
          </Link>
        </section>

        <section className="pilha conteudo">
          <h2>Conta</h2>
          {sessao.data && (
            <p>
              {sessao.data.usuario.nome}
              <br />
              <span className="secundario">{sessao.data.usuario.email}</span>
            </p>
          )}
          <button
            type="button"
            className="botao botao--contorno botao--largo"
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
          >
            Sair
          </button>
        </section>
      </div>
    </div>
  );
}
