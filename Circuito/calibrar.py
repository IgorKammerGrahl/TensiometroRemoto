#!/usr/bin/env python3
"""Ensaio de calibracao do no contra o vacuometro mecanico.

Le o CSV que o firmware imprime em MODO_CALIBRACAO, pareia cada leitura do
vacuometro digitada aqui com a mediana das ultimas amostras do no e ajusta a
reta v_sensor = v_zero + k * p. Sai com os numeros que o formulario de
calibracao do app pede e com as constantes para o firmware.

  python3 calibrar.py /dev/ttyUSB0 --vdd-mv 4870 [--dh-cm 0] [--janela 30]
  python3 calibrar.py --ajuste-de calibracao-AAAAMMDD-HHMMSS-pares.csv --vdd-mv 4870
  python3 calibrar.py --teste

Roteiro do ensaio: Circuito/README.md, "Roteiro do ensaio de calibracao".
So biblioteca padrao: nada de pyserial.
"""

import argparse
import csv
import math
import os
import statistics
import sys
import termios
import threading
import time
import tty
from datetime import datetime

COEF_COLUNA = 0.098  # kPa por cm de coluna de agua
MIN_AMOSTRAS = 5     # por ponto; com INTERVALO_CALIBRACAO_MS = 2000, 30 s dao 15
V_ZERO_NOMINAL, K_NOMINAL = 4.5, 0.04  # catalogo, so para orientar os patamares
CAMPOS = ['instante', 'p_vacuometro_kpa', 'dh_cm', 'p_topo_kpa', 'v_sensor_v',
          'fator', 'n', 'faixa_mv', 'tendencia_mv']


def ler_linha(texto):
    """(mv_pino, v_sensor, fora_da_faixa) de uma linha do MODO_CALIBRACAO, ou
    None para cabecalho, comentario e lixo do boot."""
    fora = 'FORA DA FAIXA' in texto
    campos = texto.split('<--')[0].strip().split(',')
    if len(campos) != 3:
        return None
    try:
        mv, v, _ = (float(c) for c in campos)
    except ValueError:
        return None
    return mv, v, fora


def ler_referencia(texto):
    """Leitura do vacuometro em kPa, virgula decimal aceita.

    RECUSA VALOR POSITIVO. O mostrador indica succao sem sinal, e "40"
    digitado no lugar de "-40" nao da erro nenhum no ajuste: produz uma reta
    de inclinacao trocada que parece um ensaio valido."""
    p = float(texto.strip().replace(',', '.'))
    if not -100 <= p <= 0:  # NaN tambem cai aqui
        raise ValueError('a leitura vai de -100 a 0 kPa, com sinal: -40, nao 40')
    return p


def no_topo(p_vacuometro, dh_cm):
    """Pressao no septo, onde o transdutor le.

    dh_cm e a altura do septo ACIMA da tomada do vacuometro. Vacuometro mais
    baixo, na agua, le pressao menos negativa que a do topo. NAO e a altura da
    capsula: a coluna inteira pesa igual sobre os dois instrumentos e fica fora
    da comparacao (TCC, secao 1.3.3)."""
    return p_vacuometro - COEF_COLUNA * dh_cm


def resumo_janela(vs):
    """Mediana, faixa e tendencia (mediana da 2a metade menos a da 1a) de uma
    janela de v_sensor, em V. Tendencia longe de zero = ainda equalizando: a
    mangueira atrasa a resposta, e o ponto tirado cedo demais entorta a reta."""
    meio = len(vs) // 2
    return (statistics.median(vs), max(vs) - min(vs),
            statistics.median(vs[meio:]) - statistics.median(vs[:meio]))


def ajustar(pares):
    """pares: [(p_topo_kpa, v_sensor_v)]. Minimos quadrados de v = v_zero + k*p.

    O RMSE e em kPa e na direcao em que o no converte, p = (v - v_zero) / k,
    porque e esse o erro que chega ao grafico."""
    if len(pares) < 3:
        raise ValueError(f'{len(pares)} ponto(s); o ajuste precisa de pelo menos 3')
    ps = [p for p, _ in pares]
    vs = [v for _, v in pares]
    k, v_zero = statistics.linear_regression(ps, vs)
    r2 = statistics.correlation(ps, vs) ** 2
    rmse = math.sqrt(statistics.fmean(((v - v_zero) / k - p) ** 2 for p, v in pares))
    return v_zero, k, r2, rmse


def para_firmware(v_zero, k, vdd_mv):
    """(VDD_SENSOR, V_ZERO_KPA_NOMINAL, K_VOLTS_POR_KPA_NOMINAL) que fazem o
    sketch reproduzir o ajuste. O sketch multiplica os dois coeficientes por
    VDD_SENSOR / 5,00; como o ajuste foi medido em vdd_mv, eles voltam para a
    base de 5 V, e VDD_SENSOR passa a ser o medido."""
    escala = 5000 / vdd_mv
    return vdd_mv / 1000, v_zero * escala, k * escala


def abrir_serial(porta):
    fd = os.open(porta, os.O_RDONLY | os.O_NOCTTY)
    tty.setraw(fd)
    attrs = termios.tcgetattr(fd)
    attrs[4] = attrs[5] = termios.B115200  # Serial.begin(115200) do sketch
    termios.tcsetattr(fd, termios.TCSANOW, attrs)
    return fd


class Leitor(threading.Thread):
    """Le a serial, grava TODA linha crua com horario e guarda as amostras.

    O arquivo cru e a copia de seguranca do ensaio: inclui o aquecimento, o
    lixo do boot e o que o parser recusou. O primeiro ensaio de bancada se
    perdeu por falta disso (TCC, cap. 5)."""

    def __init__(self, fd, bruto, fator):
        super().__init__(daemon=True)
        self.fd, self.bruto, self.fator = fd, bruto, fator
        self.trava = threading.Lock()
        self.amostras = []  # (monotonic, v_sensor, fora)
        self.erro = None

    def run(self):
        resto = b''
        try:
            while True:
                bloco = os.read(self.fd, 256)
                if not bloco:
                    raise OSError('a porta fechou')
                resto += bloco
                *linhas, resto = resto.split(b'\n')
                for crua in linhas:
                    self.linha(crua.decode('ascii', 'replace').strip())
        except (OSError, ValueError) as e:
            self.erro = str(e)

    def linha(self, texto):
        if not texto:
            return
        self.bruto.write(f'{datetime.now().isoformat(timespec="seconds")}\t{texto}\n')
        self.bruto.flush()
        lida = ler_linha(texto)
        if lida is None:
            return
        mv, v_impresso, fora = lida
        v = mv * self.fator / 1000  # a mesma conta do backend sobre raw_mv
        # O fator vai para o registro do app; se nao for o do firmware, todo o
        # historico reconstruido sai escalado errado. Melhor parar aqui.
        if abs(v - v_impresso) > 0.002:
            raise ValueError(f'v_sensor {v_impresso} nao bate com mv_pino {mv:.0f} x '
                             f'{self.fator}: o FATOR_DIVISOR do firmware e outro? use --fator')
        with self.trava:
            self.amostras.append((time.monotonic(), v, fora))

    def janela(self, segundos):
        limite = time.monotonic() - segundos
        with self.trava:
            return [(v, fora) for t, v, fora in self.amostras if t >= limite]


def gravar_pares(caminho, pares):
    """Reescreve o CSV inteiro a cada mudanca, via arquivo temporario: o CSV e
    sempre igual ao estado atual, inclusive depois de um `d`."""
    tmp = caminho + '.tmp'
    with open(tmp, 'w', newline='', encoding='utf-8') as f:
        escritor = csv.DictWriter(f, CAMPOS)
        escritor.writeheader()
        escritor.writerows(pares)
    os.replace(tmp, caminho)


def relatorio(pares, vdd_mv, fator):
    v_zero, k, r2, rmse = ajustar([(p['p_topo_kpa'], p['v_sensor_v']) for p in pares])
    ps = [p['p_topo_kpa'] for p in pares]
    print(f'\nAjuste com {len(pares)} pontos, de {min(ps):.1f} a {max(ps):.1f} kPa no topo.')
    print(f'{"p topo":>8} {"p ajuste":>9} {"erro":>7}   (kPa)')
    for p in pares:
        pa = (p['v_sensor_v'] - v_zero) / k
        print(f'{p["p_topo_kpa"]:8.2f} {pa:9.2f} {pa - p["p_topo_kpa"]:+7.2f}')
    if len(pares) < 5:
        print('ATENCAO: menos de 5 pontos; R2 e RMSE dizem pouco.')
    if max(ps) - min(ps) < 30:
        print('ATENCAO: faixa ensaiada estreita; a reta nao vale fora dela.')
    if k <= 0:
        print('ATENCAO: inclinacao nao positiva. O sensor sobe de 0,5 V a 4,5 V de -100 '
              'a 0 kPa; confira o sinal das leituras e a montagem.')

    print('\nFormulario de calibracao do app (tela do no):')
    print(f'  V no ponto de 0 kPa ....... {v_zero:.4f}')
    print(f'  k (V/kPa) ................. {k:.6f}')
    print(f'  fator do divisor .......... {fator}')
    print(f'  VDD do ensaio (mV) ........ {vdd_mv}')
    print(f'  R2 ........................ {r2:.5f}')
    print(f'  RMSE (kPa) ................ {rmse:.3f}')
    print(f'  nota: faixa {min(ps):.0f} a {max(ps):.0f} kPa, {len(pares)} pontos')

    vdd, v0n, kn = para_firmware(v_zero, k, vdd_mv)
    print('\nFirmware -- o registro no app NAO muda o kPa exibido; estas constantes sim:')
    print(f'  sketch.ino: const float VDD_SENSOR = {vdd:.3f};')
    print(f'  sketch.ino: const float V_ZERO_KPA_NOMINAL = {v0n:.4f};')
    print(f'  sketch.ino: const float K_VOLTS_POR_KPA_NOMINAL = {kn:.6f};')
    print(f'  config.h:   #define VDD_MV_NOMINAL  {vdd_mv}')


def sessao(args):
    base = os.path.join(args.saida, 'calibracao-' + datetime.now().strftime('%Y%m%d-%H%M%S'))
    arq_pares = base + '-pares.csv'
    bruto = open(base + '-serial.tsv', 'a', encoding='utf-8')
    leitor = Leitor(abrir_serial(args.porta), bruto, args.fator)
    leitor.start()
    print(f'serial crua -> {bruto.name}\npares      -> {arq_pares}')
    print('Enter mostra a janela; digite a leitura do vacuometro (ex.: -20); '
          'd desfaz o ultimo; fim ajusta.\n')

    pares = []
    while True:
        try:
            cmd = input('> ').strip().lower()
        except (EOFError, KeyboardInterrupt):
            cmd = 'fim'
        if leitor.erro:
            print(f'ERRO na serial: {leitor.erro}')
            break
        if cmd == 'fim':
            break
        if cmd == 'd':
            if pares:
                print(f'desfeito: {pares.pop()["p_vacuometro_kpa"]} kPa')
                gravar_pares(arq_pares, pares)
            continue

        amostras = leitor.janela(args.janela)
        if len(amostras) < MIN_AMOSTRAS:
            print(f'{len(amostras)} amostra(s) nos ultimos {args.janela} s: '
                  'o no esta em MODO_CALIBRACAO e imprimindo?')
            continue
        if any(fora for _, fora in amostras):
            print('ha amostra FORA DA FAIXA na janela: confira montagem e alimentacao')
            continue
        med, faixa, tend = resumo_janela([v for v, _ in amostras])
        print(f'n={len(amostras)}  mediana {med:.4f} V '
              f'(~{(med - V_ZERO_NOMINAL) / K_NOMINAL:.1f} kPa nominal)  '
              f'faixa {faixa * 1000:.1f} mV  tendencia {tend * 1000:+.1f} mV')
        if cmd == '':
            continue

        try:
            p_vac = ler_referencia(cmd)
        except ValueError as e:
            print(f'recusado: {e}')
            continue
        pares.append({
            'instante': datetime.now().isoformat(timespec='seconds'),
            'p_vacuometro_kpa': p_vac, 'dh_cm': args.dh_cm,
            'p_topo_kpa': round(no_topo(p_vac, args.dh_cm), 3),
            'v_sensor_v': round(med, 5), 'fator': args.fator, 'n': len(amostras),
            'faixa_mv': round(faixa * 1000, 1), 'tendencia_mv': round(tend * 1000, 1),
        })
        gravar_pares(arq_pares, pares)
        if len(pares) >= 3:
            _, _, r2, rmse = ajustar([(p['p_topo_kpa'], p['v_sensor_v']) for p in pares])
            print(f'ponto {len(pares)} gravado; ate aqui R2 {r2:.5f}, RMSE {rmse:.2f} kPa')
        else:
            print(f'ponto {len(pares)} gravado')

    if len(pares) >= 3:
        relatorio(pares, args.vdd_mv, args.fator)
    print(f'\nConfira que os arquivos tem conteudo antes de desligar:\n  wc -l {base}-*')


def refazer(args):
    """Ajuste a partir de um CSV de pares. Sem --dh-cm, vale o dh gravado em
    cada linha; com ele, p_topo e recalculado a partir da leitura do vacuometro."""
    with open(args.ajuste_de, newline='', encoding='utf-8') as f:
        linhas = list(csv.DictReader(f))
    pares = []
    for r in linhas:
        dh = float(r['dh_cm']) if args.dh_cm is None else args.dh_cm
        p_vac = float(r['p_vacuometro_kpa'])
        pares.append({**r, 'dh_cm': dh, 'p_topo_kpa': no_topo(p_vac, dh),
                      'v_sensor_v': float(r['v_sensor_v'])})
    fatores = {r['fator'] for r in linhas}
    if len(fatores) != 1:
        sys.exit(f'o CSV mistura fatores de divisor: {fatores}')
    relatorio(pares, args.vdd_mv, float(fatores.pop()))


def teste():
    assert ler_linha('3016,4.524,0.60') == (3016.0, 4.524, False)
    assert ler_linha('300,0.450,-101.25   <-- FORA DA FAIXA') == (300.0, 0.45, True)
    for lixo in ('mv_pino,v_sensor,kpa', '# VDD assumido: 5.00 V', 'ets Jun  8 2016', ''):
        assert ler_linha(lixo) is None, lixo

    assert ler_referencia('-40,5') == -40.5 and ler_referencia(' 0 ') == 0
    for ruim in ('40', '-101', 'nan', 'abc'):
        try:
            ler_referencia(ruim)
        except ValueError:
            continue
        raise AssertionError(f'aceitou {ruim!r}')

    assert math.isclose(no_topo(-40, 5), -40.49)

    # Reta conhecida com ruido de +-1 mV: o ajuste tem de devolve-la, e o RMSE
    # tem de estar em kPa (~0,02), nao em V (~0,0007).
    ruido = [0.001, -0.001, 0.0005, -0.0005, 0, 0.001, -0.001]
    pares = [(p, 4.51 + 0.0401 * p + e) for p, e in zip(range(0, -70, -10), ruido)]
    v_zero, k, r2, rmse = ajustar(pares)
    assert abs(v_zero - 4.51) < 0.002 and abs(k - 0.0401) < 0.0001, (v_zero, k)
    assert r2 > 0.9999 and 0.005 < rmse < 0.05, (r2, rmse)
    try:
        ajustar(pares[:2])
    except ValueError:
        pass
    else:
        raise AssertionError('ajustou com 2 pontos')

    # O sketch faz V_ZERO_KPA = V_ZERO_KPA_NOMINAL * VDD_SENSOR / 5,00.
    vdd, v0n, kn = para_firmware(4.51, 0.0401, 4870)
    assert math.isclose(v0n * vdd / 5.00, 4.51) and math.isclose(kn * vdd / 5.00, 0.0401)

    med, faixa, tend = resumo_janela([4.500, 4.502, 4.504, 4.506])
    assert math.isclose(med, 4.503) and math.isclose(faixa, 0.006) and tend > 0
    print('ok')


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument('porta', nargs='?', help='ex.: /dev/ttyUSB0')
    ap.add_argument('--vdd-mv', type=int, help='alimentacao do transdutor medida no multimetro')
    ap.add_argument('--dh-cm', type=float, help='altura do septo acima da tomada do vacuometro')
    ap.add_argument('--janela', type=float, default=30, help='segundos por ponto (30)')
    ap.add_argument('--fator', type=float, default=1.5, help='FATOR_DIVISOR do firmware (1.5)')
    ap.add_argument('--saida', default='.', help='pasta dos arquivos do ensaio')
    ap.add_argument('--ajuste-de', help='refaz o ajuste a partir de um CSV de pares')
    ap.add_argument('--teste', action='store_true', help='auto-verificacao, sem hardware')
    args = ap.parse_args()

    if args.teste:
        return teste()
    # Sem o VDD medido nao ha calibracao coerente: e campo obrigatorio no app e
    # base das constantes do firmware. Pedir antes evita descobrir no fim.
    if args.vdd_mv is None:
        ap.error('--vdd-mv e obrigatorio: meca a alimentacao do transdutor antes do ensaio')
    if args.ajuste_de:
        return refazer(args)
    if not args.porta:
        ap.error('informe a porta serial ou --ajuste-de')
    if args.dh_cm is None:
        args.dh_cm = 0.0
    sessao(args)


if __name__ == '__main__':
    main()
