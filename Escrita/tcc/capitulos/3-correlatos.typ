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

#cite(<correaquiroz2025>, form: "prose") desenvolveram um sistema de
irrigação por gotejamento e de monitoramento climático para estufas, com o
microcontrolador ESP32 como núcleo de processamento.

- *Metodologia:* o ESP32 lê sensores de temperatura e umidade do ar (DHT11),
  de radiação ultravioleta (GUVA-S12SD), de nível de água (HC-SR04) e um
  sensor capacitivo de umidade do solo, cuja saída analógica é lida
  diretamente pelo conversor analógico-digital e convertida em porcentagem
  por dois pontos de referência --- o sensor exposto ao ar e submerso em água.
  Os dados são exibidos em um visor local e enviados por Wi-Fi à plataforma
  Arduino Cloud, e um relé aciona a bomba de irrigação, em modo manual ou em
  modo automático, que a liga abaixo de 40% de umidade e a desliga acima de
  50% @correaquiroz2025.
- *Resultados:* em testes de campo, os autores relatam redução de 35% no
  consumo de água em relação ao método tradicional. Entre as limitações,
  registram flutuações nas leituras dos sensores atribuídas a interferência
  eletromagnética, com a necessidade de filtros de sinal, e a dependência de
  uma conexão estável à internet @correaquiroz2025.
- *Relação com o TCC:* o trabalho compartilha a plataforma computacional
  deste projeto. As diferenças estão na grandeza medida --- umidade
  volumétrica por sensor capacitivo, e não potencial matricial --- e no
  tratamento do sinal: a limitação que os autores registram, a flutuação das
  leituras sem filtragem, é a que o condicionamento e a mediana de amostras
  descritos no @cap-prototipo procuram tratar.

== IoT-Based Irrigation Control System with ESP32 for Sustainable Agriculture

O trabalho de #cite(<purnama2024>, form: "prose") desenvolveu um sistema de
controle de irrigação para lavouras de arroz, com o ESP32 e acompanhamento
remoto pela plataforma Blynk.

- *Metodologia:* um sensor de umidade do solo de três terminais é lido por uma
  entrada analógica do microcontrolador, e a sua leitura comanda uma válvula
  solenoide e uma bomba d'água; indicadores de nível complementam o controle
  do reservatório, e os dados são acompanhados em tempo real pela plataforma
  Blynk @purnama2024.
- *Resultados:* o sensor de umidade foi avaliado contra uma medida de
  referência em seis pontos, do solo seco ao encharcado, com erro de até
  1,67% @purnama2024. O trabalho não apresenta medidas de latência nem de
  perda na transmissão.
- *Relação com o TCC:* este estudo reforça a viabilidade técnica da
  telemetria por meio do ESP32. A presente proposta se diferencia ao
  delimitar o escopo estritamente ao monitoramento de precisão (telemetria),
  aprofundando-se na cadeia de aquisição do sinal analógico do transdutor de
  pressão XGZP6847A --- condicionamento, correção do conversor e preservação
  da leitura bruta para recalibração ---, cuja calibração contra o vacuômetro
  mecânico permanece por realizar (@sec-estado).

== Análise comparativa

A @tab-comparativa sintetiza as principais características dos trabalhos
analisados em comparação com a proposta deste TCC.

#figure(
  caption: [Comparação entre estudos correlatos e o projeto proposto],
  text(size: 10pt)[#table(
    columns: (1.7fr, 1.2fr, 1.15fr, 1.15fr, 1.55fr, 1.05fr),
    inset: (x: 5pt, y: 4.5pt),
    align: left + horizon,
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Autor (Ano)*], [*Foco principal*], [*Plataforma*], [*Sensor de solo*], [*Aquisição do sinal*], [*Atuação física?*],
    ),
    [#cite(<abdelmoneim2023>, form: "prose")], [Automação do sensor], [ESP32 + ThingSpeak], [Tensiômetro eletrônico], [Sensor digital (I#super[2]C)], [Não],
    [#cite(<correaquiroz2025>, form: "prose")], [Irrigação e clima em estufa], [ESP32 + Arduino Cloud], [Umidade volumétrica (capacitivo)], [Analógica, direto no ADC, sem filtragem], [Sim (bomba)],
    [#cite(<purnama2024>, form: "prose")], [Controle de irrigação], [ESP32 + Blynk], [Umidade volumétrica], [Analógica, direto no ADC, sem condicionamento descrito], [Sim (válvula e bomba)],
    [*Este trabalho (2026)*], [*Telemetria / suporte à decisão*], [*ESP32 + backend próprio*], [*Tensiômetro eletrônico*], [*Analógica, com condicionamento e ADC corrigido*], [*Não (monitoramento)*],
  )],
) <tab-comparativa>
#fonte[Elaborado pelo autor (2026).]

Conforme observado, embora existam soluções baseadas em ESP32, a maioria
delas se apoia em medições volumétricas de umidade, e o único trabalho que
automatiza a tensiometria o faz por meio de um sensor digital, que entrega ao
microcontrolador a medida já convertida internamente. Os dois trabalhos que
leem sensores analógicos o fazem diretamente no conversor do
microcontrolador, sem condicionamento de sinal descrito --- e um deles
registra justamente a flutuação das leituras por interferência como
limitação.

Convém delimitar com precisão o que este trabalho não reivindica como
contribuição. A autonomia energética por bateria e painel solar e o
armazenamento local das leituras durante indisponibilidade da rede já estão
presentes em @abdelmoneim2023 e, por isso, não seriam novidade. Neste
trabalho, nenhum dos dois foi implementado: ambos ficam registrados como
limitação do protótipo (@sec-estado), com precedente conhecido na
literatura. O diferencial
concentra-se em dois pontos. O primeiro é a aquisição analógica com
condicionamento de sinal e correção do conversor analógico-digital, que
mantém a cadeia de medição exposta e auditável, em vez de delegá-la a um
componente fechado. O segundo é o backend próprio, com modelo de autorização
por concessão de talhão, que permite o compartilhamento controlado de dados
entre o produtor e terceiros --- um recurso que as plataformas de nuvem
genéricas empregadas pelos correlatos não modelam em termos de talhão e
cultura.
