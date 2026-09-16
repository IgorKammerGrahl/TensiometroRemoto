#import "../abnt.typ": fonte, pendente

= Trabalhos correlatos

Este capítulo apresenta uma análise crítica de estudos recentes que possuem
convergência direta com o problema de pesquisa deste trabalho. O objetivo é
situar o desenvolvimento deste nó sensor de telemetria dentro do estado da
arte da Agricultura 4.0, identificando as abordagens metodológicas
predominantes, os resultados alcançados pela literatura e como a presente
proposta se diferencia ou complementa as soluções existentes.

== Internet of Things (IoT) for Soil Moisture Tensiometer Automation

O estudo de #cite(<abdelmoneim2023>, form: "prose") aborda o desenvolvimento
de um sistema de automação para tensiômetros mecânicos utilizando a
infraestrutura de Internet das Coisas.

- *Metodologia:* os autores acoplaram um sensor barométrico digital BMP180,
  de interface I#super[2]C, à câmara de ar de tensiômetros convencionais,
  cabendo a um microcontrolador ESP32-WROOM-32D a leitura e o envio dos
  dados. O nó opera em suspensão profunda (_deep sleep_), é alimentado por
  bateria de íons de lítio de 2400 mAh recarregada por painel solar e
  transmite três pontos a cada seis horas para a plataforma ThingSpeak,
  gravando as leituras em cartão microSD quando a rede está indisponível
  @abdelmoneim2023.
- *Resultados:* em validação de laboratório conduzida no CIHEAM Bari, na
  Itália, quatro unidades foram comparadas a um tensiômetro comercial JET
  FILL 2725, dotado de vacuômetro mecânico, em solo franco-siltoso ao longo
  de aproximadamente 40 dias. O protótipo acompanhou a tensão até $-80$ kPa
  com coeficiente de determinação $R^2 = "0,99"$ nos quatro ensaios, a um
  custo aproximado de 76 dólares por nó @abdelmoneim2023.
- *Relação com o TCC:* este trabalho é a principal referência técnica para a
  automação da tensiometria e fornece dois parâmetros diretamente aplicáveis
  a esta proposta: o teto prático de operação em torno de $-80$ kPa e a
  viabilidade de um nó de baixo custo. A diferença metodológica é o meio de
  aquisição: enquanto os autores empregam um sensor digital que entrega a
  medida já convertida internamente, este trabalho adota um transdutor de
  saída analógica, o que desloca para o projeto a responsabilidade pelo
  condicionamento do sinal e pela correção do conversor analógico-digital.

== IoT System with ESP32 for Smart Drip Irrigation and Climate Monitoring

#cite(<correaquiroz2025>, form: "prose") propuseram um sistema de
monitoramento climático e de solo voltado para estufas, utilizando o
microcontrolador ESP32 como núcleo de processamento.

- *Metodologia:* a arquitetura baseia-se no uso do ESP32 para coletar dados
  de múltiplos sensores e transmiti-los via Wi-Fi para um servidor em nuvem.
  O foco metodológico residiu na estabilidade da rede sem fio em ambientes
  agrícolas protegidos.
- *Resultados:* o sistema provou ser eficaz na redução de custos operacionais
  e na otimização do uso de energia elétrica.
- *Relação com o TCC:* o trabalho compartilha a mesma plataforma
  computacional (ESP32) deste projeto. A contribuição deste TCC em relação a
  este estudo correlato reside na especialização da leitura por tensiometria
  (potencial matricial), que é fisiologicamente mais precisa do que os
  sensores de umidade genéricos utilizados pelos autores.

== IoT-Based Irrigation Control System with ESP32 for Sustainable Agriculture

O trabalho de #cite(<purnama2024>, form: "prose") investigou a implementação
de um sistema de controle e monitoramento remoto focado na sustentabilidade
do manejo hídrico.

- *Metodologia:* foi desenvolvido um protótipo utilizando o ecossistema
  Arduino/ESP32 integrado a uma interface web para visualização de dados em
  tempo real.
- *Resultados:* os testes confirmaram a viabilidade de sistemas de baixo
  custo para a Agricultura Digital, apresentando baixa latência na
  transmissão de pacotes de dados via protocolos de rede leves.
- *Relação com o TCC:* este estudo reforça a viabilidade técnica da
  telemetria por meio do ESP32. A presente proposta se diferencia ao
  delimitar o escopo estritamente ao monitoramento de precisão (telemetria),
  aprofundando-se na calibração do sinal analógico do sensor de pressão
  XGZP6847A em relação ao vácuo mecânico.

== Análise comparativa

A @tab-comparativa sintetiza as principais características dos trabalhos
analisados em comparação com a proposta deste TCC.

#figure(
  caption: [Comparação entre estudos correlatos e o projeto proposto],
  text(size: 10pt)[#table(
    columns: (1.25fr, 1.3fr, 1.25fr, 1.2fr, 1.65fr, 1.15fr),
    inset: (x: 5pt, y: 4.5pt),
    align: left + horizon,
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Autor (Ano)*], [*Foco principal*], [*Plataforma*], [*Sensor de solo*], [*Aquisição do sinal*], [*Atuação física?*],
    ),
    [Abdelmoneim (2023)], [Automação do sensor], [ESP32 + ThingSpeak], [Tensiômetro eletrônico], [Sensor digital (I#super[2]C)], [Não],
    [Correa-Quiroz (2025)], [Monitoramento de estufa], [ESP32], [Umidade volumétrica], [#pendente[REF PENDENTE]], [Sim],
    [Purnama (2024)], [Sustentabilidade], [ESP32], [Umidade volumétrica], [#pendente[REF PENDENTE]], [Sim],
    [*Este trabalho (2026)*], [*Telemetria / suporte à decisão*], [*ESP32 + backend próprio*], [*Tensiômetro eletrônico*], [*Analógica, com condicionamento e ADC corrigido*], [*Não (monitoramento)*],
  )],
) <tab-comparativa>
#fonte[Elaborado pelo autor (2026).]

Conforme observado, embora existam soluções baseadas em ESP32, a maioria
delas se apoia em medições volumétricas de umidade, e o único trabalho que
automatiza a tensiometria o faz por meio de um sensor digital, que entrega ao
microcontrolador a medida já convertida internamente.

Convém delimitar com precisão o que este trabalho não reivindica como
contribuição. A autonomia energética por bateria e painel solar e o
armazenamento local das leituras durante indisponibilidade da rede já estão
presentes em @abdelmoneim2023; ambos são aqui adotados como decisões de
projeto informadas pela literatura, e não como novidade. O diferencial
concentra-se em dois pontos. O primeiro é a aquisição analógica com
condicionamento de sinal e correção do conversor analógico-digital, que
mantém a cadeia de medição exposta e auditável, em vez de delegá-la a um
componente fechado. O segundo é o backend próprio, com modelo de autorização
por concessão de talhão, que permite o compartilhamento controlado de dados
entre o produtor e terceiros --- um recurso que as plataformas de nuvem
genéricas empregadas pelos correlatos não modelam em termos de talhão e
cultura.
