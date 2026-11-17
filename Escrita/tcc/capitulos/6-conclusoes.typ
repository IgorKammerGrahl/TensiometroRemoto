#import "../abnt.typ": fonte, pendente

= Conclusões <cap-conclusoes>

Este trabalho partiu da pergunta de como desenvolver um nó sensor de
telemetria, fundamentado em Internet das Coisas e tensiometria eletrônica,
que monitore remota e continuamente a tensão da água no solo e disponibilize
esses dados em um aplicativo para apoiar a decisão de irrigar. A resposta que
os resultados sustentam tem duas partes, e a distinção entre elas é o
principal resultado do trabalho.

A primeira parte está respondida, do transdutor à interface. A cadeia foi
construída com componentes de prateleira de baixo custo e opera de ponta a
ponta nesse trecho: o condicionamento e a aquisição no ESP32, a transmissão
autenticada e idempotente, o armazenamento da grandeza bruta junto da
calibração que a interpreta, e a interface que apresenta ao produtor a
leitura, o histórico e as faixas de atenção. O acoplamento do transdutor ao
tensiômetro foi construído, mas ainda não produziu medida válida
(@sec-substituto). Em bancada, a
dispersão do sinal ficou uma ordem de grandeza abaixo do erro declarado para o
transdutor, e a transmissão operou sem perdas em regime permanente, depois de
removida a condição de rede responsável pelas perdas iniciais. Na escala para
a qual foi projetado, o serviço recebeu as leituras de mil nós simulados na
cadência do firmware sem nenhuma perda, com p99 de 7,5 ms, e continuou sem
perdas a 4000 requisições por segundo, numa única instância e em rede local
(@sec-escala). Com três cópias do serviço atrás do proxy reverso, a parada de
uma delas no meio da carga não custou nenhuma leitura (@sec-replicas).

A segunda parte permanece em aberto: se o valor exibido corresponde à tensão
da água no solo com exatidão útil ao manejo. Responder a ela exige a
calibração contra o vacuômetro mecânico e o ensaio em solo, e nenhum dos dois
foi concluído. O trabalho estabelece, portanto, a viabilidade técnica da
cadeia de medição e de transmissão, e não a exatidão da medida.
#pendente[RESULTADO PENDENTE: rever este parágrafo com o resultado da
calibração experimental.]

== Atendimento aos objetivos

A @tab-objetivos resume a situação de cada objetivo específico.

#figure(
  caption: [Situação dos objetivos específicos],
  text(size: 10pt)[#table(
    columns: (1.6fr, 1.3fr, 0.9fr),
    inset: (x: 5pt, y: 4.5pt),
    align: left + horizon,
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Objetivo específico*], [*Situação*], [*Onde*],
    ),
    [Estudar os fundamentos da tensiometria, da instrumentação e da comunicação IoT],
    [Cumprido],
    [Capítulos 2 e 3],
    [Projetar e montar o nó sensor],
    [Cumprido, em bancada],
    [@cap-prototipo],
    [Desenvolver o firmware de leitura e de conversão para kPa],
    [Em parte: leitura e conversão construídas, com coeficientes de catálogo e sem a correção de coluna de água],
    [@cap-prototipo; @sec-retaguarda; @sec-lacunas],
    [Implementar a infraestrutura de comunicação e o aplicativo, para múltiplos nós, talhões e usuários],
    [Cumprido quanto à construção; operação apenas em rede local],
    [@sec-estado; @sec-apresentacao],
    [Avaliar em bancada a estabilidade do sinal e a confiabilidade da transmissão],
    [Cumprido em bancada, com o transdutor aberto à atmosfera e sem TLS],
    [@sec-bancada],
    [Avaliar a operação com múltiplos nós e descrever o método],
    [Cumprido com nós simulados, em modo local: mil nós, três cópias do serviço e duas instalações independentes],
    [@sec-escala; @sec-replicas; @sec-implantacao],
  )],
) <tab-objetivos>
#fonte[Elaborado pelo autor (2026).]

Três ressalvas acompanham a tabela. No terceiro objetivo, o firmware converte
a leitura com os coeficientes nominais de catálogo e entrega a pressão no topo
do instrumento, e não ainda o potencial matricial no nível da cápsula
(@sec-lacunas). No quarto, a construção está completa, mas dois requisitos
não foram atendidos integralmente --- o RF04, cujo transporte HTTPS não foi
exercitado pelo nó físico, e o RNF04, sem armazenamento local durante a
indisponibilidade da rede ---, e a operação
ocorreu sempre com o servidor na rede local. No quinto, a estabilidade da
transmissão foi medida em dois ensaios contínuos, sem transporte cifrado e sem
condição adversa controlada. No sexto, os nós são simulados, e cliente e
serviço dividiram a mesma máquina, em modo local; a implantação em servidor
remoto, com certificado de autoridade pública, não foi exercitada
(@sec-implantacao). A precisão da
conversão, que a proposta original incluía no quinto objetivo, saiu dos
objetivos com o recorte descrito na @sec-objetivos: depende do ensaio de
calibração, cujo procedimento está descrito na @sec-validacao, e o RNF03, que
exige erro compatível com o uso agronômico, segue sem verificação.
#pendente[RESULTADO PENDENTE: coeficientes, coeficiente de determinação e erro
quadrático médio da calibração experimental.]

== Contribuições

No campo da computação, o trabalho contribui menos pela novidade de cada
componente --- a automação da tensiometria tem precedente direto na literatura
@abdelmoneim2023 --- do que por três propriedades do conjunto. A primeira é a
rastreabilidade da medida: a grandeza bruta é preservada junto do
identificador da calibração, e o valor exibido é convertido no servidor pela
calibração que cada leitura referencia, de modo que um ensaio novo passa a
valer sem regravar o firmware e sem reescrever o significado do histórico. A
segunda é a autorização expressa como estrutura da consulta, e não como
verificação condicional no código, com o compartilhamento de dados modelado
por concessão de talhão. A terceira é uma interface projetada para não afirmar
ao produtor um fato falso sobre o solo: leituras antigas não exibem zona,
lacunas não são interpoladas, e a cor nunca é o único canal de um alerta.

Soma-se a isso o registro explícito dos erros encontrados e corrigidos, e do
que cada ensaio não autoriza afirmar, que permite a quem retomar o trabalho
saber exatamente o que foi verificado e o que não foi.

== Limitações

As limitações estão detalhadas ao longo do @cap-prototipo e são aqui apenas
reunidas:

- a calibração experimental e a validação em solo não foram realizadas, e o
  RNF03 segue sem verificação (@sec-bancada);
- o RNF04 não está atendido: sem armazenamento local, a leitura cujo envio
  falha se perde (@sec-estado);
- a alimentação do transdutor não é medida, de modo que a calibração vale para
  a alimentação em que o ensaio for feito (@sec-retaguarda);
- o valor exibido é a pressão no topo do instrumento, sem a correção de coluna
  de água (@sec-lacunas);
- o transporte cifrado do RF04 não foi exercitado contra o servidor, e toda a operação
  ocorreu em rede local;
- o ciclo de registro de calibração, gravação no portal e aceitação da leitura
  não foi exercitado com o nó físico (@sec-provisionamento);
- a operação com múltiplos nós foi avaliada com nós simulados, que não
  exercitam o rádio, com o serviço, as cópias e as instalações na mesma
  máquina do cliente, e apenas quanto à ingestão (@sec-escala);
- com mais de uma cópia do serviço, o limite de tentativas de login por conta
  fica mais folgado, uma cota por cópia (@sec-replicas);
- não foram realizados testes com usuários representativos do produtor rural.

== Trabalhos futuros

As limitações acima apontam, em grande parte, os próprios trabalhos futuros:

- vedação do tensiômetro, a começar pela troca da rolha, e calibração contra o
  vacuômetro mecânico pelo procedimento da @sec-validacao (@sec-substituto);
- ensaio de carga com cliente e serviço em máquinas distintas, com TLS, longa
  duração e as consultas do aplicativo sob carga;
- limitador de tentativas de login no banco de dados, compartilhado pelas
  cópias do serviço, e atualização do serviço sem interrupção, uma cópia por
  vez;
- tipo de sensor como entidade do sistema, generalizando o armazenamento, as
  faixas de atenção e a apresentação, que hoje assumem o tensiômetro
  (@sec-extensao);
- concentrador local de nós por ESP-NOW ou LoRa, que guarde as leituras
  durante a falta de conexão (@sec-extensao);
- armazenamento local das leituras com retransmissão, e suspensão profunda
  entre aquisições; como o servidor já descarta retransmissões por
  idempotência, a mudança restringe-se ao nó;
- medição da alimentação do transdutor por um segundo divisor resistivo, com a
  correção ratiométrica aplicada na conversão feita pelo servidor;
- registro da profundidade de instalação no cadastro do nó, para que o servidor
  converta a leitura no topo em tensão no solo;
- hospedagem em servidor remoto com transporte cifrado, compatibilizando o
  certificado raiz embutido no firmware com a autoridade certificadora do
  servidor;
- ensaio de longa duração em campo, que produziria as primeiras séries de
  secagem real do sistema;
- estimativa do tempo restante até o limiar de atenção a partir da tendência
  da série, cuja validação depende dessas séries de secagem real;
- testes de usabilidade com produtores rurais.
