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
  // TODO: sobrenome e titulação do orientador
  orientador: "Prof. André",
  natureza: [
    Trabalho de Conclusão de Curso submetido ao curso de Bacharelado em
    Ciência da Computação do Instituto Federal Catarinense -- _Campus_ Rio do
    Sul para a obtenção do título de Bacharel em Ciência da Computação.
  ],
  // agradecimentos: [Texto dos agradecimentos aqui.],
  resumo: [
    Este trabalho descreve o desenvolvimento de um protótipo de telemetria
    baseado em Internet das Coisas (IoT) e tensiometria eletrônica para o
    monitoramento remoto da tensão da água no solo. A agricultura irrigada
    responde pela maior parcela do consumo de água doce do planeta, e a
    ineficiência no manejo hídrico compromete tanto a produtividade quanto a
    sustentabilidade ambiental. Nesse contexto, o tensiômetro é considerado
    instrumento de referência para o agendamento da irrigação, porém sua
    dependência de leituras manuais limita o uso em larga escala. Foram
    construídos os quatro estágios da cadeia de medição: um tensiômetro
    analógico derivado por septo perfurado e acoplado a um transdutor de
    pressão piezorresistivo; o condicionamento do sinal e a aquisição em um
    microcontrolador ESP32; a telemetria sobre HTTPS, com idempotência
    garantida pelo par identificador do nó e número de sequência; e um serviço
    de retaguarda com aplicação web progressiva embarcada no próprio binário,
    que apresenta ao produtor a leitura corrente, o histórico e as faixas de
    atenção configuráveis. A verificação combinou suíte automatizada de
    testes, ensaio ponta a ponta contra o servidor em execução e dois ensaios
    contínuos de bancada com o transdutor aberto à atmosfera. No segundo
    deles, de 13 h 36 min e 784 amostras, o sinal apresentou desvio padrão de
    2,6 mV, e não houve perda de pacotes em 772 emissões consecutivas após a
    correção da condição de rede responsável pelas perdas iniciais. As duas
    corridas, medindo a mesma grandeza em dias distintos, diferiram em 1,9 mV
    --- cerca de 0,07 kPa ---, o que caracteriza a reprodutibilidade entre
    ensaios como incerteza maior que a dispersão interna de cada um.
    Permanecem não realizadas a calibração experimental contra o vacuômetro
    mecânico, que manteve os coeficientes nominais de catálogo, e a validação
    em solo. O trabalho estabelece, portanto, a viabilidade técnica da cadeia
    de medição e de transmissão, e delimita explicitamente as afirmações que
    os ensaios executados não autorizam.
  ],
  palavras-chave: (
    "Internet das Coisas",
    "Tensiometria eletrônica",
    "Telemetria agrícola",
    "Irrigação de precisão",
    "ESP32",
  ),
  abstract: [
    This work describes the development of a telemetry prototype based on the
    Internet of Things (IoT) and electronic tensiometry for remote monitoring
    of soil water tension. Irrigated agriculture accounts for the largest
    share of the planet's freshwater consumption, and inefficient water
    management compromises both productivity and environmental
    sustainability. In this context, the tensiometer is considered the
    reference instrument for irrigation scheduling, but its reliance on manual
    readings limits large-scale use. All four stages of the measurement chain
    were built: an analog tensiometer tapped through a needle-pierced septum
    and coupled to a piezoresistive pressure transducer; signal conditioning
    and acquisition on an ESP32 microcontroller; telemetry over HTTPS, with
    idempotency guaranteed by the pair node identifier and sequence number;
    and a back-end service with a progressive web application embedded in the
    binary itself, which presents the farmer with the current reading, the
    history and the configurable attention thresholds. Verification combined
    an automated test suite, an end-to-end trial against the running server
    and two continuous bench trials with the transducer open to the
    atmosphere. In the second of these, lasting 13 h 36 min and comprising 784
    samples, the signal showed a standard deviation of 2.6 mV, and no packet
    loss occurred over 772 consecutive transmissions once the network
    condition responsible for the initial losses had been corrected. The two
    runs, measuring the same quantity on different days, differed by 1.9 mV
    --- about 0.07 kPa ---, which characterises between-run reproducibility as
    an uncertainty larger than the internal dispersion of either run.
    Experimental calibration against the mechanical vacuum gauge, which left
    the catalogue nominal coefficients in place, and validation in soil remain
    unperformed. The work therefore establishes the technical feasibility of
    the measurement and transmission chain, and explicitly delimits the claims
    that the trials carried out do not authorise.
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

#heading(numbering: none, outlined: true)[Referências]
#bibliography(
  "referencias.yml",
  title: none,
  style: "associacao-brasileira-de-normas-tecnicas",
  full: true,
)
