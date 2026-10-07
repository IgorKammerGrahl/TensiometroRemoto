#import "../abnt.typ": fonte, pendente

= Conclusões <cap-conclusoes>

Este trabalho partiu da pergunta de como desenvolver um nó sensor de
telemetria, fundamentado em Internet das Coisas e tensiometria eletrônica,
que monitore remota e continuamente a tensão da água no solo e disponibilize
esses dados em um aplicativo para apoiar a decisão de irrigar. A resposta que
os resultados sustentam tem duas partes, e a distinção entre elas é o
principal resultado do trabalho.

A primeira parte está respondida. A cadeia completa foi construída com
componentes de prateleira de baixo custo e opera de ponta a ponta: o
transdutor analógico acoplado ao tensiômetro, o condicionamento e a aquisição
no ESP32, a transmissão autenticada e idempotente, o armazenamento da grandeza
bruta junto da calibração que a interpreta, e a interface que apresenta ao
produtor a leitura, o histórico e as faixas de atenção. Em bancada, a
dispersão do sinal ficou uma ordem de grandeza abaixo do erro declarado para o
transdutor, e a transmissão operou sem perdas em regime permanente, depois de
removida a condição de rede responsável pelas perdas iniciais.

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
    [Cumprido],
    [@cap-prototipo; @sec-retaguarda],
    [Implementar a infraestrutura de comunicação e o aplicativo],
    [Cumprido quanto à construção; operação apenas em rede local],
    [@sec-estado; @sec-apresentacao],
    [Avaliar a precisão da conversão e a estabilidade da transmissão],
    [Em parte: transmissão avaliada em bancada; precisão da conversão pendente],
    [@sec-bancada],
  )],
) <tab-objetivos>
#fonte[Elaborado pelo autor (2026).]

Duas ressalvas acompanham a tabela. No quarto objetivo, a construção está
completa, mas dois requisitos não foram atendidos integralmente --- o RF08,
cumprido apenas na direção de configurar, e o RNF04, sem armazenamento local
durante a indisponibilidade da rede ---, e a operação ocorreu sempre com o
servidor na rede local. No quinto, a estabilidade da transmissão foi medida em
dois ensaios contínuos, sem transporte cifrado e sem condição adversa
controlada; a precisão da conversão depende do ensaio de calibração, cujo
procedimento está descrito na @sec-validacao.
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

- a calibração experimental e a validação em solo não foram realizadas
  (@sec-bancada);
- o RF08 está cumprido apenas na direção de configurar, e o RNF04 não está
  atendido (@sec-estado);
- a alimentação do transdutor não é medida, de modo que a calibração vale para
  a alimentação em que o ensaio for feito (@sec-retaguarda);
- o valor exibido é a pressão no topo do instrumento, sem a correção de coluna
  de água (@sec-lacunas);
- o transporte cifrado não foi exercitado contra o servidor, e toda a operação
  ocorreu em rede local;
- o ciclo de registro de calibração, gravação no portal e aceitação da leitura
  não foi exercitado com o nó físico (@sec-provisionamento);
- não foram realizados testes com usuários representativos do produtor rural.

== Trabalhos futuros

As limitações acima apontam, em grande parte, os próprios trabalhos futuros:

- armazenamento local das leituras com retransmissão, e suspensão profunda
  entre aquisições; como o servidor já descarta retransmissões por
  idempotência, a mudança restringe-se ao nó;
- medição da alimentação do transdutor por um segundo divisor resistivo, com a
  correção ratiométrica aplicada na conversão feita pelo servidor;
- registro da profundidade de instalação no cadastro do nó, para que o servidor
  converta a leitura no topo em tensão no solo;
- remoção da faixa de atenção pela interface, o que exige distinguir, no
  contrato da API, a chave ausente do valor nulo;
- hospedagem em servidor remoto com transporte cifrado, compatibilizando o
  certificado raiz embutido no firmware com a autoridade certificadora do
  servidor;
- ensaio de longa duração em campo, que produziria as primeiras séries de
  secagem real do sistema;
- estimativa do tempo restante até o limiar de atenção a partir da tendência
  da série, cuja validação depende dessas séries de secagem real;
- testes de usabilidade com produtores rurais.
