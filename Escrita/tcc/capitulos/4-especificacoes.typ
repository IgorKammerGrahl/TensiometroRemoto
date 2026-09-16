#import "../abnt.typ": fonte
#import "@preview/fletcher:0.5.8" as fletcher: diagram, node, edge
#import "@preview/cetz:0.3.4"

= Especificações formais

Este capítulo apresenta as especificações formais do protótipo de telemetria
para monitoramento da tensão da água no solo, contemplando a arquitetura
proposta, os requisitos funcionais e não funcionais do sistema, bem como a
modelagem conceitual dos principais casos de uso e entidades de dados. Essas
definições balizam o desenvolvimento do hardware, do firmware embarcado, da
infraestrutura em nuvem e do aplicativo de monitoramento, garantindo que a
solução atenda ao escopo delimitado e ao objetivo geral de atuar como
ferramenta de suporte à decisão no manejo da irrigação.

== Visão geral da arquitetura

A arquitetura do sistema é organizada em três camadas principais: nó sensor
de campo, infraestrutura de comunicação em nuvem e camada de apresentação ao
usuário. O nó sensor integra o tensiômetro analógico, o transdutor de pressão
XGZP6847A100KPGN e o microcontrolador ESP32, responsável pela aquisição do sinal
analógico, conversão para unidades de tensão (kPa) e envio dos dados pela
rede Wi-Fi. Na nuvem, um serviço de backend recebe as leituras, armazena-as
em um banco de dados temporal e disponibiliza uma API para consulta, enquanto
o aplicativo móvel oferece uma interface gráfica para acompanhamento em tempo
real do histórico hídrico do solo.

Essa organização em camadas favorece a modularidade e a escalabilidade do
sistema, permitindo que futuras extensões — como a inclusão de novos tipos de
sensores ou a integração com sistemas de gestão agrícola — sejam realizadas
sem alterar o núcleo de aquisição de dados. Ao mesmo tempo, a separação clara
entre telemetria e atuação física assegura que o protótipo permaneça restrito
à função de monitoramento e suporte à decisão, em conformidade com o escopo
definido.

A @fig-infraestrutura apresenta o diagrama de infraestrutura da solução,
evidenciando os elementos de hardware, software, conectividade e os agentes
envolvidos em cada camada, bem como o fluxo dos dados desde a aquisição do
sinal no campo até a visualização pelo usuário.

#figure(
  text(size: 9pt)[
    #diagram(
      spacing: (13mm, 14mm),
      node-shape: fletcher.shapes.rect,
      node-stroke: 0.6pt,
      node-corner-radius: 2pt,
      node-inset: 6pt,
      edge-stroke: 0.6pt,
      label-size: 8pt,

      // Camada 1 — nó sensor de campo
      node((0, 0), align(center)[Tensiômetro\ analógico]),
      edge((0, 0), (1, 0), "-|>", label: [sucção do solo], label-side: left),
      node((1, 0), align(center)[Transdutor de pressão\ XGZP6847A100KPGN]),
      edge((1, 0), (2, 0), "-|>", label: [sinal analógico (V)], label-side: left),
      node((2, 0), align(center)[ESP32\ #text(size: 8pt)[ADC + firmware\ (calibração V → kPa)]]),
      node(
        enclose: ((0, 0), (2, 0)),
        inset: 10pt,
        stroke: (paint: gray, dash: "dashed", thickness: 0.5pt),
        fill: none,
        align(top + left)[#move(dy: -16pt)[#text(size: 8pt, fill: gray.darken(40%))[*Nó sensor de campo (hardware)*]]],
      ),

      // Conectividade campo → nuvem
      edge((2, 0), (2, 1), "-|>", label: align(center)[rede Wi-Fi\ HTTPS (REST)], label-side: left),

      // Camada 2 — nuvem
      node((2, 1), align(center)[Backend\ #text(size: 8pt)[(ingestão de leituras)]]),
      edge((2, 1), (1, 1), "-|>", label: [gravação], label-side: left),
      node((1, 1), align(center)[Banco de dados\ temporal]),
      edge((1, 1), (0, 1), "-|>", label: [consulta], label-side: left),
      node((0, 1), align(center)[API de\ consulta]),
      node(
        enclose: ((0, 1), (2, 1)),
        inset: 10pt,
        stroke: (paint: gray, dash: "dashed", thickness: 0.5pt),
        align(top + left)[#move(dy: -16pt)[#text(size: 8pt, fill: gray.darken(40%))[*Infraestrutura em nuvem (software)*]]],
      ),

      // Conectividade nuvem → apresentação
      edge((0, 1), (0, 2), "-|>", label: [HTTPS (REST)], label-side: left),

      // Camada 3 — apresentação
      node((0, 2), align(center)[Aplicativo móvel /\ painel web]),
      edge((0, 2), (1, 2), "-|>", label: [visualização e alertas], label-side: left),
      node((1, 2), align(center)[Usuário\ #text(size: 8pt)[(produtor rural)]], shape: fletcher.shapes.pill),
      node(
        enclose: ((0, 2), (1, 2)),
        inset: 10pt,
        stroke: (paint: gray, dash: "dashed", thickness: 0.5pt),
        align(top + right)[#move(dy: -16pt)[#text(size: 8pt, fill: gray.darken(40%))[*Camada de apresentação (agentes)*]]],
      ),
    )
  ],
  caption: [Diagrama de infraestrutura da arquitetura proposta],
) <fig-infraestrutura>
#fonte[Elaborado pelo autor (2026).]

== Requisitos funcionais

Os requisitos funcionais descrevem o conjunto de funcionalidades que o
sistema deve oferecer para cumprir seu propósito de monitoramento remoto da
tensão da água no solo. A seguir, apresentam-se os principais requisitos
identificados:

- *RF01 -- Aquisição do sinal do sensor:* o sistema deve realizar leituras
  periódicas do sinal analógico proveniente do transdutor de pressão acoplado
  ao tensiômetro, utilizando uma porta ADC do microcontrolador ESP32.
- *RF02 -- Conversão para kPa:* o firmware embarcado deve aplicar uma função
  de calibração que converta o valor de tensão (V) lido em uma medida de
  pressão de sucção (kPa), correspondente ao potencial matricial da água no
  solo.
- *RF03 -- Registro de leituras com carimbo temporal:* cada leitura
  convertida deve ser associada a data e horário de aquisição, permitindo a
  reconstrução do histórico de variação da tensão da água no solo ao longo do
  tempo.
- *RF04 -- Transmissão de dados via Wi-Fi:* o nó sensor deve enviar, em
  intervalos configuráveis, os registros de leitura para o serviço em nuvem
  por meio de requisições HTTP sobre TLS (HTTPS), autenticadas por um token
  próprio do dispositivo. Optou-se pelo modelo de requisição e resposta, em
  vez de publicação e assinatura, por dispensar um intermediário permanente
  e por tornar síncrona a confirmação de gravação: o nó sensor só descarta a
  leitura do seu armazenamento local após receber do servidor o código de
  sucesso correspondente.
- *RF05 -- Armazenamento persistente em nuvem:* o backend deve armazenar as
  leituras recebidas em um repositório persistente, organizado por nó sensor,
  talhão e cultura, garantindo a integridade e a rastreabilidade dos dados.
- *RF06 -- Visualização em tempo real:* o aplicativo móvel ou painel web deve
  apresentar ao usuário, em forma de gráficos e indicadores, a tensão atual
  da água no solo e o histórico recente, permitindo a interpretação rápida
  das condições hídricas.
- *RF07 -- Consulta a histórico de medições:* o usuário deve poder selecionar
  intervalos de datas para visualizar séries históricas mais longas,
  possibilitando a análise de tendências e a comparação entre períodos de
  irrigação.
- *RF08 -- Configuração de faixas de atenção:* o sistema deve permitir o
  cadastro de limites de tensão (kPa) associados a zonas de conforto, alerta
  e estresse para cada cultura, servindo como referência visual na interface
  de monitoramento.
- *RF09 -- Notificações de alerta:* sempre que a leitura atual ultrapassar os
  limites configurados de atenção ou estresse, o sistema deve sinalizar
  visualmente essa condição no aplicativo, podendo, futuramente, ser
  estendido para o envio de notificações.
- *RF10 -- Gerenciamento de nós sensores:* o usuário deve conseguir
  cadastrar, editar e desativar nós sensores, associando cada dispositivo a
  um talhão ou área monitorada específica.

== Requisitos não funcionais

Os requisitos não funcionais estabelecem características de qualidade que
orientam a implementação do protótipo, especialmente no que diz respeito a
desempenho, usabilidade, confiabilidade e viabilidade de implantação em
pequenas propriedades rurais.

- *RNF01 -- Baixo custo:* o conjunto de componentes de hardware deve ser
  composto majoritariamente por dispositivos de prateleira de baixo custo,
  mantendo-se alinhado à proposta de democratização da Agricultura 4.0.
- *RNF02 -- Consumo energético moderado:* o nó sensor deve operar com consumo
  compatível com o uso em campo, permitindo alimentação por rede elétrica ou
  por soluções alternativas, como baterias e painéis solares, em evoluções
  futuras.
- *RNF03 -- Confiabilidade das leituras:* a calibração do sensor deve
  garantir que a conversão entre tensão e kPa apresente erro compatível com o
  uso agronômico, minimizando desvios em relação ao vacuômetro mecânico.
- *RNF04 -- Estabilidade da telemetria:* o envio de dados deve ser robusto a
  oscilações temporárias da conexão Wi-Fi, implementando estratégias de
  reenvio ou armazenamento em buffer quando a rede não estiver disponível.
- *RNF05 -- Usabilidade da interface:* o aplicativo móvel ou painel web deve
  apresentar uma interface clara, com elementos gráficos intuitivos, ícones
  legíveis e organização lógica das informações, considerando o perfil do
  produtor rural.
- *RNF06 -- Escalabilidade horizontal:* a solução em nuvem deve suportar o
  cadastro de múltiplos nós sensores e usuários, permitindo a expansão do
  sistema para diferentes talhões ou propriedades sem alteração da
  arquitetura.
- *RNF07 -- Segurança básica dos dados:* o transporte das leituras entre o nó
  sensor e a nuvem deve utilizar canais autenticados e, preferencialmente,
  criptografados, evitando o acesso indevido às informações agronômicas.
- *RNF08 -- Portabilidade da camada de apresentação:* a interface de
  monitoramento deve ser projetada de forma a facilitar sua disponibilização
  em diferentes plataformas (Android, iOS e navegadores web), garantindo
  acesso ubíquo aos dados.

== Casos de uso principais

A interação entre o produtor e o sistema de telemetria pode ser descrita por
meio de casos de uso, representando cenários típicos de operação. Os
principais atores são: o *Produtor Rural*, responsável pelo manejo da
irrigação; o *Sistema de Telemetria em Nuvem*; e o *Nó Sensor de Campo*.

- *UC01 -- Cadastrar nó sensor:* o produtor registra um novo dispositivo no
  sistema, associando-o a um talhão e informando um identificador lógico (por
  exemplo, "Tensiômetro Estufa 1"). Após o cadastro, o backend gera
  credenciais ou parâmetros de configuração para o firmware do ESP32.
- *UC02 -- Monitorar tensão da água no solo em tempo real:* ao acessar o
  aplicativo, o produtor visualiza a lista de nós ativos e, ao selecionar um
  deles, obtém a leitura atual em kPa e um gráfico com as últimas horas ou
  dias de medições.
- *UC03 -- Consultar histórico de medições:* o produtor escolhe um intervalo
  de datas e visualiza o comportamento da tensão da água no solo ao longo
  desse período, podendo comparar diferentes fases fenológicas ou ciclos de
  irrigação.
- *UC04 -- Configurar faixas de atenção:* o produtor define valores de tensão
  correspondentes a zonas de conforto, alerta e estresse para a cultura
  monitorada; esses limites passam a ser exibidos como linhas de referência
  sobre os gráficos de leitura.
- *UC05 -- Receber alertas visuais:* ao ultrapassar as faixas de atenção ou
  estresse, o sistema destaca imediatamente o nó sensor em estado de alerta
  na interface, auxiliando o produtor a priorizar possíveis intervenções no
  manejo hídrico.

A @fig-casos-uso representa esses casos de uso em notação UML. O ator
Produtor Rural relaciona-se diretamente com os casos UC01 a UC05, enquanto o
nó sensor de campo, ator externo ao sistema de telemetria, participa
exclusivamente do envio de leituras.

#figure(
  caption: [Diagrama de casos de uso do sistema de telemetria],
  block[
  #cetz.canvas({
    import cetz.draw: *
    set-style(stroke: 0.6pt)

    let ator(x, y, nome) = {
      circle((x, y + 0.72), radius: 0.22)
      line((x, y + 0.5), (x, y - 0.25))
      line((x - 0.38, y + 0.24), (x + 0.38, y + 0.24))
      line((x, y - 0.25), (x - 0.3, y - 0.8))
      line((x, y - 0.25), (x + 0.3, y - 0.8))
      content((x, y - 1.15), align(center)[#text(size: 9pt)[#nome]])
    }

    let uc(x, y, txt) = {
      circle((x, y), radius: (2.5, 0.62))
      content((x, y), align(center)[#text(size: 8.5pt)[#txt]])
    }

    // fronteira do sistema
    rect((3.0, -7.5), (11.0, 1.4), stroke: (dash: "dashed", paint: gray.darken(30%)))
    content((7.0, 1.75), text(size: 9pt, style: "italic")[Sistema de telemetria em nuvem])

    ator(0.9, -3.0, [Produtor\ rural])
    ator(12.9, -6.3, [Nó sensor\ de campo])

    uc(6.2, 0.6, [UC01 --- Cadastrar nó sensor])
    uc(6.2, -0.8, [UC02 --- Monitorar tensão\ da água no solo])
    uc(6.2, -2.2, [UC03 --- Consultar histórico\ de medições])
    uc(6.2, -3.6, [UC04 --- Configurar faixas\ de atenção])
    uc(6.2, -5.0, [UC05 --- Receber alertas visuais])
    uc(6.2, -6.6, [Enviar leituras de tensão])

    for y in (0.6, -0.8, -2.2, -3.6, -5.0) {
      line((1.3, -3.0), (3.7, y))
    }
    line((12.4, -6.3), (8.7, -6.6))
  })
  ],
) <fig-casos-uso>
#fonte[Elaborado pelo autor (2026).]


== Modelo de dados

Do ponto de vista lógico, o sistema baseia-se em um modelo de dados centrado
no histórico de leituras de tensão da água no solo, associado a nós sensores
e áreas monitoradas. As principais entidades conceituais são:

- *Usuário:* representa o produtor rural ou outro responsável pelo
  monitoramento; possui atributos como identificador, nome e credenciais de
  acesso.
- *Talhão:* descreve a área ou parcela da lavoura monitorada, contendo
  informações sobre localização, cultura plantada e, opcionalmente,
  características de solo relevantes.
- *NóSensor:* representa o conjunto físico tensiômetro + transdutor + ESP32
  instalado em um talhão; inclui identificador, descrição, status
  (ativo/inativo) e parâmetros de calibração.
- *LeituraTensiométrica:* registra cada medição de tensão da água no solo,
  contendo valor em kPa, timestamp, identificador do nó sensor e,
  opcionalmente, metadados como temperatura ambiente ou umidade relativa.
- *FaixaAtencao:* armazena limites de tensão configurados para uma
  determinada cultura ou talhão, definindo patamares de conforto, alerta e
  estresse hídrico.

As relações principais incluem: um Usuário pode gerenciar vários Talhões;
cada Talhão pode conter um ou mais NósSensores; cada NóSensor possui muitas
LeiturasTensiométricas; e cada Talhão pode ter uma ou mais FaixasAtencao
associadas. Esse modelo favorece consultas históricas eficientes e a expansão
futura do sistema para outras métricas ambientais.

== Fluxo de operação do sistema

O fluxo de operação do protótipo pode ser descrito em etapas sequenciais,
desde a aquisição do dado no campo até a visualização no aplicativo:

+ O tensiômetro analógico, instalado no solo, gera uma variação de pressão de
  sucção conforme a água é extraída ou reposta no perfil do solo.
+ O transdutor XGZP6847A100KPGN converte a pressão na câmara de ar do
  tensiômetro em um sinal elétrico analógico, na faixa de 0,5 V a 4,5 V,
  correspondente à faixa de pressão de $-100$ a $0$ kPa em relação inversa:
  quanto menor a pressão, menor a tensão de saída.
+ O ESP32 lê periodicamente esse sinal por meio de uma porta ADC, aplica a
  calibração e obtém o valor correspondente de tensão da água no solo em kPa.
+ O firmware organiza os dados em mensagens contendo valor, timestamp e
  identificador do nó sensor, enviando-os pela conexão Wi-Fi ao serviço em
  nuvem por meio de requisição HTTPS autenticada.
+ O backend recebe as mensagens, registra as leituras em banco de dados e as
  torna acessíveis por meio de uma API.
+ O aplicativo móvel consulta periodicamente a API, atualiza os gráficos e
  indicadores, e destaca visualmente leituras que ultrapassem as faixas de
  atenção configuradas pelo usuário.

O @fig-sequencia detalha esse fluxo em notação de sequência, evidenciando as
interações entre os componentes e os pontos de validação de dados, incluindo
o comportamento do sistema sob retransmissão.

== Plano preliminar de testes

Os testes previstos para o protótipo concentram-se em três frentes:
verificação da calibração analógica, avaliação da confiabilidade da
telemetria e validação da interface de monitoramento. Em ambiente de bancada
ou estufa em pequena escala, serão realizados ensaios de secagem e
umedecimento do solo, comparando-se as leituras do sistema com o vacuômetro
mecânico, de forma a quantificar o erro de conversão tensão--kPa.

Adicionalmente, serão monitoradas métricas como taxa de perda de pacotes,
latência média de transmissão e estabilidade da conexão Wi-Fi entre o nó
sensor e a nuvem. Na camada de apresentação, testes exploratórios com
usuários representativos do perfil do produtor rural serão empregados para
verificar clareza das informações, facilidade de navegação e tempo necessário
para identificar situações de alerta hídrico.

Das três frentes previstas, duas foram executadas e uma não. A confiabilidade
da telemetria foi medida em dois ensaios contínuos e a interface foi
verificada por suíte automatizada e por ensaio ponta a ponta contra o servidor
em execução, ambos relatados no @cap-prototipo. A verificação da calibração
analógica não foi realizada, e com ela ficaram por fazer os ensaios de secagem
e umedecimento do solo; os testes exploratórios com usuários também não foram
conduzidos. As limitações decorrentes estão declaradas ao final daquele
capítulo.

O plano de testes deve, ainda, incorporar três condicionantes de natureza
metrológica identificadas durante o desenvolvimento. A comparação entre a
leitura eletrônica e a leitura do vacuômetro exige a correção de coluna de
água descrita na seção 1.3.3, sob pena de atribuir ao transdutor um desvio que
é, na verdade, hidrostático. A faixa de ensaio deve limitar-se ao intervalo em
que a coluna de água permanece íntegra, uma vez que além do limite de
cavitação a leitura do próprio padrão de referência deixa de ser válida
@azevedo1999. E a exatidão atribuível ao protótipo é limitada pela do
instrumento de referência, de modo que os ensaios estabelecem concordância, e
não exatidão absoluta.

O capítulo seguinte descreve o estágio de desenvolvimento efetivamente
alcançado até a presente etapa, relatando as decisões de projeto tomadas, os
erros de especificação identificados e corrigidos, os resultados de bancada
já obtidos e, de forma explícita, o que ainda não pode ser afirmado a partir
deles.
