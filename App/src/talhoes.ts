import type { DeviceApp, Talhao } from './api';
import { situacao } from './leitura';

// Panorama por talhao: a lista de nos agrupada, para quando ela tem centenas.
//
// NADA AQUI ORDENA. O servidor manda os nos do mais seco para o mais umido
// (ORDER BY kpa ASC), e agrupar preservando essa ordem da duas coisas sem
// nenhum .sort(): dentro de cada talhao, o no mais seco primeiro; e entre os
// talhoes, primeiro o que tem o no mais seco. Ordenar aqui seria a segunda
// implementacao da regra de sinal invertido -- ver o comentario em Nos.tsx.

export type ContagemTalhao = {
  conforto: number;
  alerta: number;
  estresse: number;
  /** Leitura atual num talhao sem limites: ha valor, nao ha classificacao. */
  semFaixa: number;
  /** Velha, nunca reportou ou desativado: o estado de agora e desconhecido. */
  semLeitura: number;
};

export type GrupoTalhao = {
  talhao: Talhao;
  /** Na ordem do servidor. */
  nos: readonly DeviceApp[];
  contagem: ContagemTalhao;
  /** kPa das leituras ATUAIS, um traco cada na tira. Leitura velha nao entra:
   *  um traco na tira afirma onde o solo esta agora. */
  leituras: readonly number[];
  simulados: number;
};

export function agruparPorTalhao(
  devices: readonly DeviceApp[],
  fetchadoEmMs: number,
  agoraMs: number,
): GrupoTalhao[] {
  // Map preserva a ordem de insercao: e ela que ordena os talhoes.
  const grupos = new Map<string, GrupoTalhao & { nos: DeviceApp[]; leituras: number[] }>();
  for (const d of devices) {
    let g = grupos.get(d.talhao.id);
    if (!g) {
      g = {
        talhao: d.talhao,
        nos: [],
        contagem: { conforto: 0, alerta: 0, estresse: 0, semFaixa: 0, semLeitura: 0 },
        leituras: [],
        simulados: 0,
      };
      grupos.set(d.talhao.id, g);
    }
    g.nos.push(d);
    if (d.simulado) g.simulados++;

    const s = situacao(d.ativo, d.ultima, fetchadoEmMs, agoraMs);
    if (s.tipo !== 'atual') {
      g.contagem.semLeitura++;
      continue;
    }
    g.leituras.push(s.kpa);
    // A zona vem classificada do servidor; aqui so se conta.
    if (s.zona === null) g.contagem.semFaixa++;
    else g.contagem[s.zona]++;
  }
  return [...grupos.values()];
}

/** Sem acento e sem caixa: "pivo" acha "pivô", "talhao" acha "Talhão". */
const normalizar = (t: string) =>
  t.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase().trim();

/** Filtra por descricao, id ou nome do talhao. Preserva a ordem recebida. */
export function buscarNos(devices: readonly DeviceApp[], termo: string): readonly DeviceApp[] {
  const t = normalizar(termo);
  if (t === '') return devices;
  return devices.filter((d) =>
    [d.descricao, d.id, d.talhao.nome].some((campo) => normalizar(campo).includes(t)),
  );
}
