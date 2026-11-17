import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { ErroAPI, SEM_REDE } from '../api';
import { useLogin } from '../sessao';

/** Mensagem para o usuario a partir do erro de login.
 *
 * 401 TEM UMA MENSAGEM SO. O servidor responde igual para senha errada e para
 * conta que nao existe -- de proposito, para nao entregar quais e-mails estao
 * cadastrados. Escrever "esse e-mail nao existe" aqui desfaria no cliente uma
 * decisao tomada no servidor, e o atacante leria a diferenca na tela.
 *
 * 403 e outra coisa e merece outro texto: a credencial estava certa. */
function mensagemDeLogin(erro: unknown): string {
  if (!(erro instanceof ErroAPI)) return 'Não foi possível entrar.';
  if (erro.status === 401) return 'E-mail ou senha incorretos.';
  if (erro.status === 403) {
    return 'Esta conta está desativada. Procure quem administra o sistema.';
  }
  if (erro.status === SEM_REDE) {
    return 'Não foi possível falar com o servidor. Confira a internet do celular.';
  }
  return erro.message;
}

export function Entrar() {
  const [email, setEmail] = useState('');
  const [senha, setSenha] = useState('');
  const [ver, setVer] = useState(false);
  const navegar = useNavigate();
  const local = useLocation();
  const login = useLogin();

  const destino =
    (local.state as { de?: string } | null)?.de ?? '/';

  function enviar(evento: React.FormEvent) {
    evento.preventDefault();
    // A senha nao fica exposta enquanto o pedido vai e volta.
    setVer(false);
    login.mutate(
      { email, senha },
      { onSuccess: () => navegar(destino, { replace: true }) },
    );
  }

  return (
    <main className="tela-borda">
      <header className="entrar__marca">
        <h1>Tensio</h1>
        <p className="secundario">Umidade do solo nos seus talhões</p>
      </header>

      <form className="pilha entrar__forma" onSubmit={enviar} noValidate>
        <div className="campo">
          <label htmlFor="email">E-mail</label>
          <input
            id="email"
            name="email"
            type="email"
            inputMode="email"
            autoComplete="username"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="next"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>

        <div className="campo">
          <label htmlFor="senha">Senha</label>
          <div className="campo__com-botao">
            <input
              id="senha"
              name="senha"
              type={ver ? 'text' : 'password'}
              autoComplete="current-password"
              enterKeyHint="done"
              required
              value={senha}
              onChange={(e) => setSenha(e.target.value)}
            />
            {/* MOSTRAR A SENHA (guia autofill-sign-in-form). Para quem digita
                devagar num teclado pequeno, conferir o que digitou e o que
                evita a terceira tentativa errada. O texto do botao diz a ACAO
                ("Mostrar"/"Esconder"), mais claro para quem enxerga que um
                botao pressionado; aria-pressed da o estado a quem ouve. O
                pointerdown sem default mantem o foco -- e o teclado aberto --
                no campo. */}
            <button
              type="button"
              className="botao botao--contorno"
              aria-controls="senha"
              aria-pressed={ver}
              onPointerDown={(e) => e.preventDefault()}
              onClick={() => setVer((v) => !v)}
            >
              {ver ? 'Esconder' : 'Mostrar'}
            </button>
          </div>
        </div>

        {/* aria-live: o erro aparece sem mudar de tela, entao um leitor de tela
            nao teria como saber que algo mudou. */}
        <div aria-live="assertive">
          {login.isError && (
            <p className="aviso">{mensagemDeLogin(login.error)}</p>
          )}
        </div>

        <button
          type="submit"
          className="botao botao--largo"
          disabled={login.isPending}
        >
          {login.isPending ? 'Entrando…' : 'Entrar'}
        </button>
      </form>
    </main>
  );
}
