import { describe, expect, it } from 'vitest';
import { ErroAPI, SEM_REDE } from './api';
import { explicarFalhaDeNo } from './erros';

const erro = (status: number, mensagem = 'mensagem do servidor') =>
  new ErroAPI(status, '/devices', mensagem);

describe('explicarFalhaDeNo', () => {
  it('separa ausencia de resposta de recusa do servidor', () => {
    const m = explicarFalhaDeNo(erro(SEM_REDE, 'sem conexao'), 'cadastro');
    expect(m).toMatch(/sem conexão/i);
    // Nao pode afirmar que a operacao foi recusada: o servidor pode nem ter
    // sido alcancado.
    expect(m).not.toMatch(/recus/i);
  });

  // O 404 de PATCH /devices/{id} cobre tres situacoes que o servidor nao
  // distingue: no inexistente, concessao de origem ausente, concessao de
  // destino ausente. Se algum dia esta asserção estiver no caminho de uma
  // mensagem mais especifica, o problema e a mensagem: a informacao que ela
  // afirmaria nao chegou na resposta.
  it('no 404 da edicao nomeia no e talhao, sem escolher entre eles', () => {
    const m = explicarFalhaDeNo(erro(404, 'device nao encontrado'), 'edicao');
    expect(m).toMatch(/nó/);
    expect(m).toMatch(/talhão/);
    expect(m).toMatch(/ou/);
  });

  it('no 404 do cadastro fala do talhao, que e o unico envolvido', () => {
    const m = explicarFalhaDeNo(erro(404, 'talhao nao encontrado'), 'cadastro');
    expect(m).toMatch(/talhão/);
    // O no ainda nao existe; culpa-lo seria descrever um estado impossivel.
    expect(m).not.toMatch(/\bnó\b/);
  });

  it('traduz o conflito de id sem repetir o vocabulario do banco', () => {
    const m = explicarFalhaDeNo(erro(409, 'ja existe device com esse id'), 'cadastro');
    expect(m).toMatch(/já existe/i);
    expect(m).not.toMatch(/device/i);
  });

  it('deixa o 400 passar como veio, porque so o servidor sabe qual campo caiu', () => {
    expect(explicarFalhaDeNo(erro(400, 'descricao ausente'), 'cadastro')).toBe(
      'descricao ausente',
    );
  });

  it('nao leva a mensagem interna do 500 para a tela', () => {
    const m = explicarFalhaDeNo(erro(500, 'pgx: connection refused'), 'edicao');
    expect(m).not.toMatch(/pgx/);
  });

  it('erro que nao e da API tambem tem resposta', () => {
    expect(explicarFalhaDeNo(new TypeError('x'), 'edicao')).toMatch(/de novo/i);
  });
});
