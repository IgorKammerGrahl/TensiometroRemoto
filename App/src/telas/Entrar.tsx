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
    return 'Não foi possível falar com o servidor. Verifique a rede do aparelho.';
  }
  return erro.message;
}

export function Entrar() {
  const [email, setEmail] = useState('');
  const [senha, setSenha] = useState('');
  const navegar = useNavigate();
  const local = useLocation();
  const login = useLogin();

  const destino =
    (local.state as { de?: string } | null)?.de ?? '/';

  function enviar(evento: React.FormEvent) {
    evento.preventDefault();
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
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>

        <div className="campo">
          <label htmlFor="senha">Senha</label>
          <input
            id="senha"
            name="senha"
            type="password"
            autoComplete="current-password"
            required
            value={senha}
            onChange={(e) => setSenha(e.target.value)}
          />
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
