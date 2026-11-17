#import "abnt.typ": tcc, fonte

#show: tcc.with(
  titulo: [Desenvolvimento de um protótipo de irrigação inteligente baseado em
    Internet das Coisas e tensiometria eletrônica para monitoramento do
    potencial hídrico do solo],
  autor: "Igor Kammer Grahl",
  instituicao: "Instituto Federal Catarinense",
  curso: "Bacharelado em Ciência da Computação",
  campus: "Rio do Sul",
  cidade: "Rio do Sul",
  ano: "2026",
  // TODO: titulação do orientador (Me./Dr.)
  orientador: "Prof. André Alessandro Stein",
  natureza: [
    Trabalho de Conclusão de Curso submetido ao curso de Bacharelado em
    Ciência da Computação do Instituto Federal Catarinense -- _Campus_ Rio do
    Sul para a obtenção do título de Bacharel em Ciência da Computação.
  ],
  // agradecimentos: [Texto dos agradecimentos aqui.],
  resumo: [
    Este trabalho descreve o desenvolvimento de um sistema de telemetria
    baseado em Internet das Coisas (IoT) e tensiometria eletrônica para o
    monitoramento remoto da tensão da água no solo, projetado para operar com
    múltiplos nós sensores. A agricultura irrigada responde pela maior parcela
    do consumo de água doce do planeta, e o tensiômetro, instrumento de
    referência para o agendamento da irrigação, depende de leituras manuais
    que limitam o seu uso em larga escala. Foram construídos um nó sensor, com
    um tensiômetro analógico acoplado a um transdutor de pressão
    piezorresistivo e a aquisição em um microcontrolador ESP32, configurado em
    campo por portal cativo; a telemetria autenticada, com idempotência
    garantida pelo par identificador do nó e número de sequência; e um serviço
    de retaguarda com aplicação web progressiva embarcada, que apresenta ao
    produtor a leitura corrente, o histórico e as faixas de atenção, e que
    converte cada leitura pela calibração que ela referencia. Em decisão
    tomada com o orientador, o escopo foi recortado: a calibração do
    instrumento, que depende da estanqueidade do tensiômetro, problema de
    natureza hidráulica, passou a esforço secundário, e o trabalho passou a
    avaliar a operação do sistema com múltiplos nós. Em bancada, com o
    transdutor aberto à atmosfera e sem TLS, o sinal apresentou desvio padrão
    de 2,6 mV ao longo de 13 h 36 min, duas corridas em dias distintos
    diferiram em 1,9 mV, cerca de 0,07 kPa, e não houve perda em 772 emissões
    consecutivas após a correção da condição de rede responsável pelas perdas
    iniciais. A operação com múltiplos nós foi avaliada com nós simulados,
    marcados como tais, que se comunicam com o servidor pela mesma interface do
    firmware: mil nós na cadência real foram atendidos sem perda e com
    latência de 7,5 ms no percentil 99, e o serviço não perdeu leitura até
    4000 requisições por segundo, sem que o seu teto fosse alcançado. Com três
    cópias do serviço atrás de um proxy reverso, a parada de uma delas no meio
    da carga, por HTTPS, não custou nenhuma de 18 000 leituras, e duas
    instalações independentes na mesma máquina mostraram-se isoladas entre si.
    O método de implantação de uma instalação e de um nó novos foi descrito
    como roteiro reprodutível. Permanecem não realizadas a calibração contra o
    vacuômetro mecânico, que manteve os coeficientes nominais de catálogo
    porque o instrumento disponível não reteve vácuo, a validação em solo e a
    implantação em servidor remoto. O trabalho estabelece, portanto, a
    viabilidade técnica da cadeia de aquisição e de transmissão e a sua
    operação com múltiplos nós em ambiente controlado, e delimita
    explicitamente as afirmações que os ensaios executados não autorizam.
  ],
  palavras-chave: (
    "Internet das Coisas",
    "Tensiometria eletrônica",
    "Telemetria agrícola",
    "Irrigação de precisão",
    "ESP32",
  ),
  abstract: [
    This work describes the development of a telemetry system based on the
    Internet of Things (IoT) and electronic tensiometry for remote monitoring
    of soil water tension, designed to operate with multiple sensor nodes.
    Irrigated agriculture accounts for the largest share of the planet's
    freshwater consumption, and the tensiometer, the reference instrument for
    irrigation scheduling, relies on manual readings that limit its
    large-scale use. The following were built: a sensor node, with an analog
    tensiometer coupled to a piezoresistive pressure transducer and
    acquisition on an ESP32 microcontroller, configured in the field through a
    captive portal; authenticated telemetry, with idempotency guaranteed by
    the pair node identifier and sequence number; and a back-end service with
    an embedded progressive web application, which presents the farmer with
    the current reading, the history and the attention thresholds, and which
    converts each reading through the calibration it references. In a
    decision taken with the advisor, the scope was narrowed: calibration of
    the instrument, which depends on the tensiometer being airtight, a
    hydraulic problem, became a secondary effort, and the work turned to
    evaluating the operation of the system with multiple nodes. On the bench,
    with the transducer open to the atmosphere and without TLS, the signal
    showed a standard deviation of 2.6 mV over 13 h 36 min, two runs on
    different days differed by 1.9 mV, about 0.07 kPa, and no loss occurred
    over 772 consecutive transmissions once the network condition responsible
    for the initial losses had been corrected. Operation with multiple nodes
    was evaluated with simulated nodes, flagged as such, which talk to the
    server through the same interface as the firmware: a thousand nodes at the
    real rate were served without loss and with a 99th-percentile latency of
    7.5 ms, and the service lost no reading up to 4000 requests per second,
    without its ceiling being reached. With three copies of the service behind
    a reverse proxy, stopping one of them in the middle of the load, over
    HTTPS, cost none of 18,000 readings, and two independent installations on
    the same machine proved isolated from each other. The deployment method
    for a new installation and a new node was described as a reproducible
    procedure. Calibration against the mechanical vacuum gauge, which kept the
    catalogue nominal coefficients because the available instrument did not
    hold vacuum, validation in soil and deployment on a remote server remain
    unperformed. The work therefore establishes the technical feasibility of
    the acquisition and transmission chain and its operation with multiple
    nodes in a controlled environment, and explicitly delimits the claims that
    the trials carried out do not authorise.
  ],
  keywords: (
    "Internet of Things",
    "Electronic tensiometry",
    "Agricultural telemetry",
    "Precision irrigation",
    "ESP32",
  ),
)

#include "capitulos/1-introducao.typ"
#include "capitulos/2-fundamentacao.typ"
#include "capitulos/3-correlatos.typ"
#include "capitulos/4-especificacoes.typ"
#include "capitulos/5-prototipo.typ"
#include "capitulos/6-conclusoes.typ"

#heading(numbering: none, outlined: true)[Referências]
#bibliography(
  "referencias.yml",
  title: none,
  style: "associacao-brasileira-de-normas-tecnicas",
  full: true,
)
