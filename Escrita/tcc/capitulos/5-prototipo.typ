#import "../abnt.typ": fonte, pendente
#import "@preview/cetz:0.3.4"
#import "@preview/fletcher:0.5.8" as fletcher: diagram, node, edge

= Desenvolvimento do protótipo <cap-prototipo>

Este capítulo relata o estágio de desenvolvimento alcançado até a presente
etapa. Diferentemente dos capítulos anteriores, que apresentam a proposta em
termos de requisitos e planejamento, aqui são descritos os artefatos
efetivamente construídos, as decisões de projeto tomadas durante a
construção, os erros identificados e corrigidos, e --- com igual destaque ---
aquilo que os resultados obtidos ainda não autorizam afirmar.

== Visão geral do estágio alcançado <sec-estado>

Quatro artefatos foram construídos e encontram-se em funcionamento
verificável. O primeiro é o circuito de condicionamento de sinal, montado em
matriz de contatos, que acopla a saída analógica do transdutor de pressão à
entrada do conversor analógico-digital do ESP32. O segundo é o firmware de
aquisição, responsável pela leitura filtrada do sinal, pela conversão para
unidades de pressão e pela transmissão autenticada ao servidor. O terceiro é o
serviço de retaguarda, que recebe, valida, armazena e disponibiliza as
leituras, com modelo de autorização por concessão de talhão. O quarto é a
camada de apresentação, uma aplicação web progressiva embutida no próprio
binário do serviço, que exibe ao produtor a leitura corrente, o histórico e as
faixas de atenção. Com ela, o quarto objetivo específico deste trabalho ---
implementar a infraestrutura de comunicação e desenvolver o aplicativo de
monitoramento --- está cumprido integralmente.

A derivação física do instrumento foi obtida, e o diagnóstico anterior a
respeito do vazamento que a acometia foi retificado: a causa era a cápsula
cerâmica dessaturada, e não a montagem improvisada. Ainda assim, toda a
experimentação relatada neste capítulo é de bancada, com o transdutor aberto à
atmosfera ou submetido a sucção manual.

Três itens previstos permanecem não realizados, e determinam os limites do que
este capítulo autoriza afirmar.

A *calibração experimental* não foi conduzida. Os coeficientes de conversão em
uso são os valores nominais de catálogo, e nenhuma métrica de ajuste ---
coeficiente de determinação, erro quadrático médio, erro percentual --- pode
ser apresentada. Com a retenção de vácuo restabelecida, esse ensaio deixou de
estar bloqueado, mas não foi realizado a tempo.

A *validação em solo* não foi conduzida. Os ensaios mantiveram o transdutor
aberto à atmosfera, condição que exercita a cadeia de aquisição, de conversão
e de transmissão, mas não a resposta do instrumento à dinâmica de secagem e
umedecimento do solo.

O *RF08 está cumprido apenas na direção de configurar.* A interface permite
definir e alterar os limiares de atenção de um talhão, mas não removê-los: o
controle que fazia isso reportava sucesso sem produzir efeito, pela razão
técnica detalhada na @sec-apresentacao, e foi retirado em vez de mantido.
Devolver um talhão ao estado não configurado exige, hoje, intervenção direta
no banco de dados. O requisito não está atendido, e o conserto, que é de
pequeno porte e pertence ao serviço de retaguarda, fica registrado como
trabalho futuro.

#pendente[FOTO PENDENTE: vista geral da montagem em matriz de contatos, com
ESP32, divisor resistivo e transdutor.]

== Especificação do transdutor e correção de escopo

O componente inicialmente adquirido para o protótipo foi o XGZP6847A100KPG.
A verificação documental realizada antes da montagem revelou que esse
componente é incompatível com a aplicação, e a razão está inteiramente contida
no sufixo do código de encomenda.

O XGZP6847A designa uma família de transdutores cujas variantes compartilham o
encapsulamento, a alimentação e a faixa de tensão de saída, mas diferem quanto
à faixa e ao tipo de pressão medida. O sufixo `G` identifica pressão
manométrica positiva; `GN`, pressão negativa, isto é, vácuo; e `GPN`, faixa
bidirecional @cfsensor. O componente adquirido, de sufixo `G`, opera na faixa
de $0$ a $100$ kPa --- pressões acima da atmosférica. O tensiômetro, por
princípio de funcionamento, jamais produz pressão positiva: a secagem do solo
gera sucção, e a pressão na câmara de ar do instrumento é sempre igual ou
inferior à atmosférica. Acoplado ao tensiômetro, o componente adquirido
permaneceria indefinidamente no extremo inferior da sua faixa, produzindo uma
tensão de saída constante e insensível à variação que se pretendia medir.

O registro deste episódio é deliberado. Trata-se de um erro de especificação,
não de montagem, e a sua detecção decorreu de conferência documental prévia,
não de ensaio. Caso tivesse sido descoberto apenas na fase de testes, todo o
trabalho experimental construído sobre esse componente estaria invalidado, e a
correção exigiria repetir a montagem, a calibração e os ensaios. A troca pela
variante XGZP6847A100KPGN, de faixa $-100$ a $0$ kPa, restabeleceu a coerência
entre o instrumento e o transdutor.

As características do componente adotado, relevantes para as decisões de
projeto descritas nas seções seguintes, são: alimentação de 5 V; saída
analógica de 0,5 V a 4,5 V; erro máximo de ±2% do fundo de escala;
tempo de resposta de 2,5 ms; e comportamento ratiométrico, isto é, tensão de
saída proporcional à tensão de alimentação @cfsensor. A correspondência entre
tensão e pressão é inversa em relação à dos transdutores de pressão positiva:
0,5 V corresponde a $-100$ kPa e 4,5 V corresponde a $0$ kPa, o que conduz à
relação de conversão

$ p = (V_"saída" - "4,5") / "0,04" $

com $p$ em kPa e $V_"saída"$ em volts, sendo $"0,04"$ V/kPa a sensibilidade
nominal declarada em catálogo @cfsensor. A @fig-curva representa essa relação,
indicando também o ponto observado em bancada.

#figure(
  caption: [Curva nominal de conversão do transdutor XGZP6847A100KPGN],
  block[
  #cetz.canvas({
    import cetz.draw: *
    set-style(stroke: 0.6pt)
    let x0 = 0; let x1 = 9; let y0 = 0; let y1 = 5.4

    // eixos
    line((x0, y0), (x1 + 0.4, y0), mark: (end: "stealth"), stroke: 0.7pt)
    line((x0, y0), (x0, y1 + 0.4), mark: (end: "stealth"), stroke: 0.7pt)
    content((x1 / 2, -1.15), text(size: 9.5pt)[Pressão (kPa)])
    content((-1.5, y1 / 2), text(size: 9.5pt)[Tensão de\ saída (V)], angle: 90deg)

    // marcas eixo x: -100 .. 0
    for i in range(0, 6) {
      let px = x0 + i * (x1 - x0) / 5
      line((px, y0), (px, y0 - 0.15))
      content((px, y0 - 0.45), text(size: 9pt)[#(-100 + i * 20)])
    }
    // marcas eixo y: 0.5 .. 4.5
    for i in range(0, 5) {
      let py = y0 + i * (y1 - y0) / 4
      line((x0, py), (x0 - 0.15, py))
      content((x0 - 0.62, py), text(size: 9pt)[#(("0,5","1,5","2,5","3,5","4,5").at(i))])
    }

    // reta de conversao
    line((x0, y0), (x1, y1), stroke: 1.2pt)
    circle((x0, y0), radius: 0.09, fill: black)
    circle((x1, y1), radius: 0.09, fill: black)
    content((1.75, 0.72), text(size: 9pt)[$-100$ kPa; 0,5 V])
    content((7.05, 4.98), text(size: 9pt)[0 kPa; 4,5 V])

    // ponto de bancada
    let vb = 4.524
    let xb = x1 - 0.05
    line((xb, y0), (xb, y1 + 0.05), stroke: (dash: "dashed", paint: gray))
    content((7.4, 2.2), text(size: 9pt, fill: rgb("#555"))[leitura de bancada:\ 4,524 V $approx$ 0 kPa])

    content((4.5, 2.35), angle: 31deg, text(size: 9.5pt)[$p = (V_"saída" - "4,5") slash "0,04"$])
  })
  ],
) <fig-curva>
#fonte[Elaborado pelo autor (2026), a partir de CFSensor (2024).]


Um segundo ponto exigiu procedimento específico antes da energização. O
diagrama de pinagem do datasheet não informa se a vista apresentada é
superior ou inferior do encapsulamento, ambiguidade que admite duas
interpretações opostas para a ordem dos terminais. Como a inversão entre
alimentação e terra em um componente energizado a 5 V pode provocar
aquecimento e dano permanente, a ordem dos terminais foi determinada
experimentalmente por teste de continuidade antes de qualquer ligação à fonte.

== Condicionamento do sinal e aquisição

A saída do transdutor alcança 4,5 V, ao passo que as entradas analógicas do
ESP32 operam em 3,3 V. A adequação é feita por um divisor resistivo composto
por uma resistência de 10 k$Omega$ entre a saída do sensor e o ponto de
leitura, e por 20 k$Omega$ entre o ponto de leitura e o terra, de modo que a
tensão no pino corresponde a dois terços da tensão do sensor. Com a saída no
extremo superior, o pino recebe 3,0 V, dentro da faixa admissível. A
reconstrução no firmware faz o caminho inverso, multiplicando a tensão lida
pelo fator 1,5.

O arranjo completo é apresentado na @fig-condicionamento.

#figure(
  caption: [Esquema do circuito de condicionamento do sinal],
  block[
  #cetz.canvas({
    import cetz.draw: *
    set-style(stroke: 0.6pt)

    rect((0, -0.55), (3.5, 1.55), name: "sensor")
    content((1.75, 0.85), align(center)[Transdutor])
    content((1.75, 0.3), align(center)[#text(size: 9pt)[XGZP6847A100KPGN]])
    content((1.75, -1.0), text(size: 9pt)[saída de 0,5 V a 4,5 V])

    line((3.5, 0.5), (4.3, 0.5))
    rect((4.3, 0.28), (5.9, 0.72))
    content((5.1, 1.2), text(size: 9pt)[R1 = 10 k$Omega$])

    line((5.9, 0.5), (11.4, 0.5))
    circle((6.9, 0.5), radius: 0.07, fill: black)
    content((8.55, 1.32), text(size: 9pt)[ponto de leitura])
    line((8.05, 1.2), (7.05, 0.68), stroke: 0.4pt)

    rect((11.4, -0.55), (14.6, 1.55))
    content((13.0, 0.85), align(center)[ESP32])
    content((13.0, 0.3), align(center)[#text(size: 9pt)[GPIO34 --- ADC1]])

    // R2 + R3 ao terra
    line((6.9, 0.5), (6.9, -0.75))
    rect((6.68, -0.75), (7.12, -2.15))
    content((5.35, -1.45), text(size: 9pt)[R2 + R3\ = 20 k$Omega$])
    line((6.9, -2.15), (6.9, -2.75))

    // Capacitor de 100 nF
    line((9.3, 0.5), (9.3, -1.3))
    line((8.85, -1.3), (9.75, -1.3))
    line((8.85, -1.6), (9.75, -1.6))
    content((10.7, -1.45), text(size: 9pt)[C = 100 nF])
    line((9.3, -1.6), (9.3, -2.75))

    // Terra comum
    line((5.6, -2.75), (10.6, -2.75))
    content((11.9, -2.75), text(size: 9pt)[terra comum])
    line((7.6, -2.95), (8.6, -2.95))
    line((7.85, -3.15), (8.35, -3.15))
    line((8.1, -2.75), (8.1, -2.95))

  })
  ],
) <fig-condicionamento>
#fonte[Elaborado pelo autor (2026).]

Esse fator foi objeto de um erro identificado e corrigido durante os ensaios
de bancada: a primeira versão do firmware empregava o valor 3,0, resultado de
confundir a razão entre a resistência inferior e a superior com a razão entre
a tensão do sensor e a tensão do pino. O sintoma foi uma leitura de pressão
coerente em forma, porém deslocada, o que ilustra uma característica
incômoda desse tipo de defeito: um fator de escala errado não produz saída
inválida, produz saída plausível.

Um capacitor de 100 nF liga o ponto de leitura ao terra, formando um filtro
passa-baixas com a resistência equivalente vista pelo capacitor. Essa
resistência é o paralelo entre os dois braços do divisor, 10 k$Omega$ $parallel$ 20 k$Omega$ $approx$ 6,67 k$Omega$,
o que resulta em constante de tempo $tau approx "0,67"$ ms. Comparada ao tempo de resposta de 2,5 ms
declarado para o transdutor @cfsensor, a constante de tempo do filtro é
aproximadamente quatro vezes menor. Conclui-se que o filtro não é o elemento
limitante da resposta do sistema: a velocidade com que uma variação de pressão
se reflete na leitura é determinada pelo transdutor, e não pelo
condicionamento. O filtro cumpre a função de atenuar ruído de alta frequência
sem introduzir atraso significativo na grandeza de interesse, que em
tensiometria varia na escala de horas.

A entrada escolhida é o GPIO34, pertencente ao primeiro conversor
analógico-digital do microcontrolador. A escolha não é preferência de projeto,
mas decorrência de uma restrição documentada: o segundo conversor é também
utilizado pelo subsistema de rádio, e o acesso a ele durante a operação do
Wi-Fi está sujeito a contenção, com o driver retornando erro de expiração
quando o recurso está ocupado @espressifadc. Em um nó cuja função é justamente
transmitir por Wi-Fi, depender desse conversor introduziria uma falha
intermitente de aquisição sem causa aparente no código de aplicação.

A leitura emprega a função `analogReadMilliVolts()`, que aplica os parâmetros
de calibração gravados de fábrica na memória eFuse do chip, em vez da
conversão proporcional ingênua a partir do valor bruto. A distinção é
relevante porque a tensão de referência do conversor é projetada para 1100 mV
mas varia entre aproximadamente 1000 mV e 1200 mV de exemplar para exemplar,
sendo os parâmetros de correção específicos de cada chip queimados em eFuse
durante a fabricação @espressifcali. Uma conversão que assuma o valor nominal
incorpora, portanto, um erro sistemático de até cerca de 9%, que nenhuma
calibração posterior do conjunto conseguiria distinguir do comportamento do
transdutor.

Cada leitura publicada é a mediana de 31 amostras consecutivas, substituindo
uma versão inicial que empregava a média de 50 amostras. A troca responde a
uma característica documentada do conversor, descrito pelo fabricante como
sensível a ruído, com discrepâncias expressivas entre leituras sucessivas
@espressifadc. A média é adequada contra ruído de distribuição simétrica, mas
é deslocada por picos espúrios isolados, ao passo que a mediana rejeita ambos
os efeitos; a redução no número de amostras é possível justamente porque a
mediana não depende do tamanho da amostra para descartar valores atípicos.

#pendente[FOTO PENDENTE: detalhe do divisor resistivo e do capacitor de
desacoplamento na matriz de contatos.]

== Arquitetura de telemetria

A especificação inicial mantinha em aberto a escolha entre um protocolo de
publicação e assinatura e um protocolo de requisição e resposta. A decisão
adotada foi HTTP sobre TLS, com interface no estilo REST.

A literatura que compara protocolos para a Internet das Coisas atribui ao MQTT
menor sobrecarga por mensagem e melhor adequação a cenários de muitos
consumidores para a mesma fonte de dados @naik2017. Nenhuma dessas duas
vantagens se realiza no cenário deste trabalho. A frequência de transmissão em
tensiometria é baixa, da ordem de poucas leituras por hora, de modo que a
economia de bytes por mensagem é irrelevante frente ao custo de manter o
rádio ativo; e cada leitura tem um único destino, o serviço de retaguarda, o
que elimina o benefício da distribuição por tópicos. Em contrapartida, o
modelo de publicação e assinatura exige um intermediário permanente, que
constitui um componente adicional a operar, monitorar e proteger.

O fator decisivo, contudo, é o acoplamento entre confirmação e descarte. Na
requisição e resposta, o nó sensor obtém de forma síncrona o código de estado
correspondente à gravação, e só então descarta a leitura do seu armazenamento
local. A confirmação não é de entrega ao intermediário, mas de persistência no
destino final.

A duplicidade decorrente de retransmissão é tratada por idempotência: cada
leitura carrega um número de sequência monotônico atribuído pelo dispositivo,
e o par formado pelo identificador do dispositivo e por esse número constitui
a chave primária da tabela de leituras. Uma retransmissão da mesma leitura é
identificada e descartada pelo próprio banco de dados, sem depender de lógica
de aplicação. Quando uma transmissão falha em definitivo, o número de
sequência avança mesmo assim, por decisão de projeto: a lacuna resultante é
diagnosticável na série, ao passo que reaproveitar o número produziria duas
leituras distintas sob o mesmo identificador.

A @fig-sequencia detalha esse comportamento em três situações sucessivas: a
transmissão bem-sucedida, a perda da resposta após gravação efetiva e a
retransmissão subsequente, descartada por idempotência.

#figure(
  caption: [Sequência de ingestão de leituras, com retransmissão e descarte idempotente],
  block[
  #cetz.canvas({
    import cetz.draw: *
    set-style(stroke: 0.6pt)
    let xa = 0.9; let xb = 6.3; let xc = 11.6
    let top = 0; let bot = -11.9

    for (x, rot) in ((xa, [Nó sensor]), (xb, [API de ingestão]), (xc, [Banco de dados])) {
      rect((x - 1.5, top), (x + 1.5, top + 0.85))
      content((x, top + 0.42), text(size: 9.5pt)[#rot])
      line((x, top), (x, bot), stroke: (dash: "dashed", paint: gray.darken(20%)))
    }

    let msg(y, x1, x2, txt, dashed: false) = {
      line((x1, y), (x2, y), mark: (end: "stealth"),
           stroke: if dashed { (dash: "dashed") } else { 0.6pt })
      content(((x1 + x2) / 2, y + 0.3), text(size: 8.5pt)[#txt])
    }
    let fase(y, txt) = {
      content((xa - 1.5, y), anchor: "west", text(size: 8.5pt, style: "italic", fill: rgb("#444"))[#txt])
    }

    fase(-0.75, "1. Primeira transmissão")
    msg(-1.5, xa, xb, [POST /leituras --- seq = #box[$n$]])
    msg(-2.35, xb, xc, [INSERT ... ON CONFLICT DO NOTHING])
    msg(-3.1, xc, xb, [1 linha inserida], dashed: true)
    msg(-3.85, xb, xa, [201 Created], dashed: true)
    content((xa, -4.35), text(size: 8.5pt)[descarta a leitura do buffer local])

    fase(-5.35, "2. Falha de rede — resposta não recebida")
    msg(-6.1, xa, xb, [POST /leituras --- seq = #box[$n + 1$]])
    msg(-6.45, xb, xc, [INSERT --- gravada com sucesso])
    line((xb, -7.4), (xa, -7.4), stroke: (dash: "dashed"), mark: (end: "stealth"))
    content(((xa + xb) / 2, -7.1), text(size: 8.5pt)[201 Created (perdida em trânsito)])
    line((3.0, -7.15), (4.2, -7.7), stroke: 1.1pt)
    line((3.0, -7.7), (4.2, -7.15), stroke: 1.1pt)
    content((xa, -8.15), text(size: 8.5pt)[mantém a leitura no buffer])

    fase(-9.0, "3. Retransmissão — descarte idempotente")
    msg(-9.75, xa, xb, [POST /leituras --- seq = #box[$n + 1$] (repetida)])
    msg(-10.5, xb, xc, [INSERT ... ON CONFLICT DO NOTHING])
    content(((xb + xc) / 2, -10.85), text(size: 8.5pt)[0 linhas --- a chave (dispositivo, seq) já existe])
    msg(-11.55, xb, xa, [200 OK --- leitura já registrada], dashed: true)
  })
  ],
) <fig-sequencia>
#fonte[Elaborado pelo autor (2026).]

O armazenamento local durante indisponibilidade da rede, previsto para a etapa
seguinte, tem precedente direto na literatura, em que se emprega cartão de
memória para o mesmo fim @abdelmoneim2023. Trata-se, portanto, de decisão de
projeto informada pela literatura, e não de contribuição original deste
trabalho.

Um detalhe de ordem de inicialização merece registro por não ser evidente. O
firmware sincroniza o relógio por NTP antes de estabelecer qualquer conexão
TLS. A razão é que a validação do certificado do servidor compara a validade
declarada contra o relógio local: com o relógio na origem da contagem, em
1970, um certificado perfeitamente válido é recusado sob a alegação de ainda
não haver sido emitido. O certificado raiz da autoridade certificadora é
embutido no firmware, de modo que a validação não depende de nenhum
repositório externo de confiança.

== Serviço de retaguarda e modelo de dados

O serviço de retaguarda foi implementado em Go, utilizando a biblioteca padrão
para o roteamento HTTP e três dependências externas: o driver de acesso ao
PostgreSQL, o utilitário de migrações de esquema e a biblioteca criptográfica
complementar. A contenção deliberada no número de dependências responde a uma
preocupação de manutenibilidade em um projeto de autoria única e horizonte
longo, em que cada dependência representa uma superfície de atualização e uma
possível fonte de incompatibilidade futura.

A decisão de projeto mais consequente no modelo de dados diz respeito ao que
se armazena junto de cada leitura. Além do valor de pressão em kPa, são
persistidos a tensão bruta lida no pino, em milivolts, a tensão de alimentação
do sensor e o identificador do conjunto de coeficientes de calibração vigente
no momento da aquisição. A motivação é direta: os coeficientes atualmente
empregados são os valores nominais de catálogo, e serão substituídos pelos
valores obtidos em calibração experimental. Armazenar apenas o resultado
convertido tornaria todo o histórico anterior à calibração definitivamente
inutilizável; armazenando a grandeza bruta e a identificação dos coeficientes,
o histórico pode ser integralmente reprocessado quando os coeficientes forem
refinados.

A @fig-modelo-dados apresenta as entidades e os relacionamentos do modelo.

#figure(
  caption: [Modelo de entidades e relacionamentos do serviço de retaguarda],
  block[
  #let ent(nome, campos) = align(left, box(inset: 0pt)[
    #text(weight: "bold", size: 9.5pt)[#nome]
    #v(-0.45em)
    #line(length: 100%, stroke: 0.5pt)
    #v(-0.35em)
    #text(size: 8.5pt)[#campos]
  ])

  #diagram(
    spacing: (13mm, 11mm),
    node-stroke: 0.6pt,
    node-shape: rect,
    node-inset: 6pt,
    node((0, 0), ent("usuarios", [id\ email\ senha_hash]), name: <u>),
    node((1, 0), ent("usuario_talhoes", [usuario_id\ talhao_id\ papel]), name: <ut>),
    node((2, 0), ent("talhoes", [id\ nome\ cultura]), name: <t>),
    node((3, 0), ent("dispositivos", [id\ talhao_id (nulo)\ token_hash]), name: <d>),
    node((3, 1), ent("leituras", [dispositivo_id (PK)\ seq (PK)\ kpa / raw_mv\ vdd_mv\ calibracao_id\ medido_em\ recebido_em]), name: <l>),
    node((0, 1), ent("sessoes", [token_hash\ usuario_id\ expira_em]), name: <s>),
    node((1, 1), ent("calibracoes", [id\ v_zero_kpa\ k_volts_por_kpa]), name: <c>),

    edge(<u>, <ut>, "-|>", label: text(size: 8pt)[1..N]),
    edge(<ut>, <t>, "<|-", label: text(size: 8pt)[N..1]),
    edge(<t>, <d>, "-|>", label: text(size: 8pt)[1..N]),
    edge(<d>, <l>, "-|>", label: text(size: 8pt)[1..N], label-side: right),
    edge(<u>, <s>, "-|>", label: text(size: 8pt)[1..N], label-side: right),
    edge(<c>, <l>, "-|>", label: text(size: 8pt)[1..N]),
  )
  ],
) <fig-modelo-dados>
#fonte[Elaborado pelo autor (2026).]

Cada leitura registra dois instantes distintos: aquele em que a medição foi
realizada, informado pelo dispositivo, e aquele em que o servidor a recebeu. A
diferença entre ambos é, por construção, a métrica de latência de telemetria
prevista no plano de testes, e o seu registro sistemático dispensa
instrumentação adicional para obtê-la.

== Modelo de autenticação e autorização <sec-autorizacao>

O sistema possui dois planos de autenticação, separados por rota e sujeitos a
requisitos opostos. O primeiro autentica dispositivos na ingestão de leituras;
o segundo autentica pessoas no acesso à consulta.

A credencial de dispositivo é verificada por HMAC-SHA256 com um segredo mantido
no servidor, sem função de derivação lenta. A escolha é justificada por duas
propriedades da credencial: ela é gerada aleatoriamente com 256 bits de
entropia, o que a torna imune a ataque de dicionário, e é verificada a cada
requisição de ingestão, de modo que uma função deliberadamente lenta
inviabilizaria a busca indexada e transformaria o mecanismo de proteção em
gargalo.

A credencial de usuário é tratada de forma oposta, com bcrypt e fator de custo
12. Senhas escolhidas por pessoas têm entropia baixa e previsível, o que as
expõe a ataque de dicionário; a lentidão deixa de ser um custo e passa a ser a
propriedade desejada, e o fator de custo ajustável permite acompanhar a
evolução do poder computacional sem alterar o formato armazenado
@provos1999. O volume de verificações, restrito aos momentos de autenticação,
torna esse custo irrelevante para o desempenho do sistema.

A autorização segue um princípio estrutural: ela é expressa como cláusula de
junção na consulta, e não como verificação condicional no código de aplicação.
O acesso a uma leitura decorre de uma cadeia --- do dispositivo ao talhão, do
talhão à concessão, da concessão ao usuário --- e essa cadeia é percorrida
dentro da própria consulta ao banco. A consequência prática é que omitir a
verificação de autorização não produz uma falha silenciosa de segurança, mas
uma consulta que não compila ou não retorna, porque a coluna de vínculo com o
usuário deixaria de estar disponível.

Duas decisões complementam esse modelo. Recursos fora do alcance do usuário
respondem com o código de estado 404, e nunca 403: informar que um recurso
existe mas é inacessível revela a sua existência a quem não deveria conhecê-la,
convertendo o mecanismo de autorização em oráculo de enumeração. E um
dispositivo ainda não associado a um talhão é invisível a todos os usuários,
sem exceção, uma vez que a cadeia de junção não se fecha; a adoção de um
dispositivo órfão só é possível por via administrativa, fora do fluxo comum da
aplicação. Trata-se de comportamento deliberadamente restritivo: na ausência
de vínculo explícito, o padrão é negar.

== Faixas de atenção e tratamento do sinal <sec-faixas>

A conversão de uma série de valores de tensão da água no solo em orientação
utilizável exige classificá-los em faixas. O modelo adotado define dois
limiares e deriva três zonas por comparação, em vez de definir três faixas
independentes. A diferença não é estilística: com três faixas declaradas
separadamente, tornam-se representáveis configurações em que existem lacunas
entre elas, ou sobreposições, e essas configurações precisariam ser detectadas
e rejeitadas por validação. Derivando as zonas de dois limiares por comparações
sucessivas, lacunas e sobreposições tornam-se irrepresentáveis por construção,
e a validação correspondente deixa de ser necessária.

O tratamento do sinal algébrico é o ponto de maior propensão a erro em toda a
aplicação. A tensão da água no solo é uma grandeza negativa, e valores mais
negativos indicam solo mais seco, isto é, situação mais severa. Toda a
intuição associada a limiares --- de que exceder um limiar significa
ultrapassá-lo para cima --- está invertida. O limiar de $-60$ kPa é mais
severo que o de $-30$ kPa, embora seja numericamente menor. Em consequência, as
comparações que classificam uma leitura percorrem os limiares em ordem inversa
à que a leitura ingênua do código sugeriria, e essa inversão é a primeira
candidata a erro em qualquer alteração futura --- razão pela qual recebe
cobertura específica de testes, conforme descrito na seção seguinte.

O comportamento nas fronteiras exatas é definido explicitamente: um valor
igual a um limiar pertence à zona menos severa. A regra é arbitrária no
sentido de que a escolha oposta seria igualmente defensável, mas deixar o
comportamento de fronteira indefinido não é uma opção, porque produz
classificações que variam conforme o caminho de código percorrido.

A classificação é realizada no servidor, e não na camada de apresentação. Duas
razões sustentam essa escolha: a classificação é regra de negócio e deve
produzir o mesmo resultado independentemente do cliente que consome os dados;
e a existência de múltiplos clientes futuros multiplicaria as implementações
da mesma regra, com risco de divergência. Os limiares em si também são
retornados junto da classificação, para que a interface possa desenhar as
linhas de referência sem precisar reconstruí-los.

Por fim, a agregação temporal das leituras para exibição em séries mais longas
emprega o valor mínimo de cada intervalo, e não a média. Em uma grandeza cuja
severidade cresce no sentido negativo, a média dilui exatamente o que importa
detectar: um período curto de estresse hídrico severo, diluído em uma média
diária, desaparece. O mínimo preserva o pior caso do intervalo, que é a
informação relevante para a decisão de irrigar.

== Camada de apresentação <sec-apresentacao>

A camada de apresentação foi construída. Trata-se de uma aplicação web
progressiva em React com TypeScript, servida pelo próprio binário do serviço
de retaguarda.

A justificativa da tecnologia está no requisito de portabilidade entre
plataformas. Uma aplicação web progressiva atende aos ambientes desktop e
móvel com um único artefato, sem duplicação de código de interface e sem
processo de publicação em lojas de aplicativos --- o que, no caso da
plataforma iOS, evitaria também o custo recorrente de uma conta de
desenvolvedor. A análise das funcionalidades previstas não identificou nenhuma
dependência de recurso nativo do dispositivo: a aplicação consulta séries
históricas e exibe classificações calculadas no servidor, operações
inteiramente cobertas pelas capacidades de um navegador.

O que se segue não é uma descrição de telas. São as decisões de projeto em que
a interface pode afirmar ao produtor um fato falso sobre o solo dele, e a
forma como cada uma foi tratada.

=== Distribuição em artefato único

Os arquivos compilados da aplicação são embutidos no executável do serviço em
tempo de compilação, de modo que a implantação consiste em um único binário,
sem servidor de arquivos estáticos separado e sem possibilidade de divergência
de versão entre a interface e a API que ela consome.

O roteamento decorrente exigiu uma decisão explícita. O multiplexador atende
dois espaços de nomes na mesma origem: o prefixo da API e todo o restante, que
devolve o documento da aplicação para que o roteamento do lado do cliente
funcione em qualquer caminho. A regra é que *uma rota desconhecida sob o
prefixo da API responde em JSON, e nunca com o documento da aplicação.* A
alternativa --- deixar o caminho genérico capturar também os endereços de API
não encontrados --- produziria uma falha silenciosa de diagnóstico
particularmente ruim: um endereço digitado errado responderia com código de
sucesso e conteúdo HTML, e o cliente falharia ao interpretar o documento como
dados, reportando um erro de sintaxe em vez de um recurso inexistente.

Um defeito apareceu apenas nessa configuração de distribuição. O manifesto da
aplicação era servido com o tipo de conteúdo errado, porque a extensão
`.webmanifest` não consta da tabela padrão da biblioteca de tipos da
linguagem, e o valor de reserva é texto simples. O sintoma é a aplicação
deixar de se oferecer para instalação, sem erro em nenhum log e sem falha em
nenhuma requisição --- apenas um recurso da plataforma que nunca aparece. A
correção é declarar o tipo explicitamente antes de servir o arquivo.

=== O sinal invertido, pela quarta vez

A inversão de sinal na comparação com limiares negativos, tratada na
@sec-faixas no firmware e no serviço de retaguarda, reapareceria naturalmente
na interface: bastaria a tela decidir a cor a partir dos limiares do talhão. A
decisão de projeto foi *não repetir a defesa, e sim tornar o erro
inexprimível.*

A função que traduz zona em aparência recebe a zona já classificada pelo
servidor, e nada mais. A sua assinatura não aceita limiar algum. Não existe,
portanto, forma de escrever nesse módulo a comparação entre leitura e limiar
--- nem correta, nem invertida --- porque os operandos não estão disponíveis.
A classificação tem uma única origem, que é a mesma consultada pela ordenação
das listas e pelo resumo textual do gráfico.

O mesmo princípio governa o formulário de configuração dos limiares (RF08). O
formulário não verifica se o limiar de estresse é menor que o de alerta:
ele torna impossível construir o par invertido, cruzando os limites nativos
dos campos numéricos --- o piso de um campo é o valor corrente do outro. Não
há comparação escrita entre os dois valores em lugar nenhum do formulário; há
um intervalo do qual o navegador não deixa sair. A validação propriamente dita
permanece onde já estava, no serviço e na restrição de integridade da tabela,
e a mensagem de erro do servidor é exibida como veio, porque só ela sabe qual
restrição falhou. Escrever a comparação no formulário criaria uma segunda
fonte de verdade sobre a mesma regra, que divergiria da primeira sem aviso.

Duas condições que o mecanismo nativo não cobre continuam tratadas no
servidor, e o registro é deliberado: a igualdade exata entre os dois valores,
porque os limites do campo são inclusivos enquanto a restrição exige
desigualdade estrita; e a chegada dos valores por qualquer caminho que não
seja aquele formulário.

=== Estados que não podem colapsar em conforto

A interface distingue quatro situações, verificadas nesta ordem: o nó está
desativado; o nó nunca reportou; a última leitura é antiga demais para
descrever o presente; a leitura é atual. *A zona de atenção só é alcançável no
último caso.*

A ordem é o mecanismo. Um nó desativado que não reporta está em silêncio
esperado, e não em falha --- tratá-lo como falha treinaria o produtor a
ignorar o alerta. Um nó que nunca reportou é distinto de um que parou de
reportar, porque implica ação diferente: o primeiro sugere erro de instalação
ou de credencial, o segundo sugere falta de energia ou de enlace. E a
verificação de idade precede a de zona, e não o contrário: uma leitura de
horas atrás pode estar em qualquer faixa, e exibir a faixa dela é afirmar
sobre o agora um fato que se refere ao passado.

A régua de escala explicita a distinção entre medir e classificar. Ela tem
dois elementos separáveis: a escala de $0$ a $-80$ kPa com a posição da
leitura nela, que é medição pura e não depende de limiar; e as bandas de
conforto, alerta e estresse, que dependem dos dois limiares do talhão. Um
talhão sem faixa configurada perde as bandas e *mantém* a escala e a posição:
o produtor continua lendo se está perto de zero ou perto do fundo de escala,
sem saber em que zona isso cai para a cultura dele. A alternativa considerada
--- suprimir a régua inteira --- foi descartada porque seria o único ponto da
interface em que a ausência de configuração removeria um componente em vez de
marcá-lo, e porque descartaria junto uma informação que o servidor de fato
enviou.

Nenhum desses estados é comunicado apenas por cor. Cada um tem posição, forma,
padrão de preenchimento e rótulo textual próprios, e a cor é redundante em
relação a todos eles. A justificativa está no RNF05 lido em concreto: a tela
será consultada sob luz solar direta, onde a distinção entre matizes se
degrada, e por um usuário que pode ser daltônico. Nas duas situações a cor
deixa de ser um canal confiável, e um sistema cujo alerta depende só dela
falha exatamente para quem mais precisa dele.

=== Descontinuidade da série

Uma lacuna no histórico não pode ser desenhada como uma reta. A reta entre dois
pontos distantes no tempo é uma afirmação sobre o que aconteceu no intervalo,
e essa afirmação é inventada.

A garantia não vem de uma biblioteca. O gráfico é SVG construído diretamente,
e cada trecho contíguo da série é emitido como um elemento de caminho
independente. A propriedade "o gráfico não ligou a linha através da lacuna"
torna-se, assim, verificável contando elementos no documento, e não
interpretando a cadeia de coordenadas de um único caminho. A biblioteca de
formas que havia sido cogitada na etapa anterior foi removida do projeto: as
suas curvas suavizadas interpolam entre amostras, o que é o mesmo erro da
lacuna ligada em escala menor --- afirmar valores que não foram medidos ---, e
a interpolação linear honesta cabe em uma linha de código. Restou apenas a
biblioteca de escalas, usada para as marcações do eixo de tempo.

A regra de segmentação é escrita em uma forma específica e a forma importa:
toda condição pergunta *"estes pontos são contíguos?"*, nunca *"existe uma
lacuna aqui?"*. A diferença aparece quando o intervalo entre dois pontos não é
um número --- carimbo temporal malformado, ordenação incorreta, parâmetro
inválido. Um valor não numérico reprova toda comparação, então a pergunta pela
contiguidade responde "não" e o trecho é quebrado, enquanto a pergunta pela
lacuna responderia "não há lacuna" e a linha seria ligada. *O modo de falha
escolhido é um gráfico fragmentado, que é visível, em vez de uma reta
inventada, que não é.*

=== Três defeitos que só apareceram contra o sistema real

Os três defeitos a seguir passaram pela suíte automatizada e pela verificação
contra o dublê de desenvolvimento. Todos apareceram na primeira execução
contra o sistema completo, e o que os une é que o dublê, por ser uma
simplificação escrita por quem escreveu o cliente, reproduzia as suposições do
cliente em vez de contradizê-las.

*Lacuna inventada onde o dado estava íntegro.* O contrato da API expõe a
resolução com que o servidor agrega as leituras em intervalos, e esse
parâmetro foi tomado como o período de amostragem do dispositivo. Contra o
dublê, que emitia exatamente um ponto por intervalo de agregação, as duas
grandezas coincidiam e o erro era invisível. Contra dados reais de um nó que
reporta a cada quinze minutos, com agregação de sessenta segundos, todo par de
pontos consecutivos dista quinze minutos e a regra declarava lacuna em todos
os intervalos: uma série contínua de 77 pontos foi fragmentada em 77 trechos
de um ponto cada, e o resumo anunciou 76 interrupções inexistentes. *É o
inverso exato do erro que a segmentação existe para impedir* --- em vez de
inventar dado onde faltou, inventou falta onde o dado estava inteiro. A
correção passou a derivar o período da própria série, pela mediana dos
intervalos observados. Mediana e não média: a média seria elevada justamente
pelas lacunas que se pretende detectar, de modo que uma série com poucas
interrupções longas passaria a tolerá-las como se fossem o ritmo normal.

*Controle que reporta sucesso sem produzir efeito.* O formulário de limiares
possuía um botão para remover a faixa configurada, devolvendo o talhão ao
estado não classificado. Ele respondia com código de sucesso e não alterava
nada. A causa está na desserialização do lado do servidor: os campos são
ponteiros, e a ausência da chave no documento recebido e a sua presença com
valor nulo produzem o mesmo ponteiro nulo, indistinguíveis entre si; a
atualização interpreta o ponteiro nulo como "preserve o valor atual", de modo
que pedir a remoção equivale a não pedir nada. *O controle foi removido, e não
mantido.* O princípio é o mesmo que governa o gráfico e o indicador de estado:
uma interface que afirma um fato falso é pior que uma funcionalidade ausente,
porque a ausência é percebida e a falsidade não. O conserto pertence ao
serviço de retaguarda, é pequeno e está registrado; enquanto não for feito, o
RF08 está cumprido apenas na direção de configurar, e essa limitação está
declarada na @sec-estado.

*Invalidação de cache incompleta, visível apenas por leitor de tela.* O resumo
textual do gráfico --- o texto alternativo que um leitor de tela anuncia ---
é construído a partir da classificação de cada ponto, que vem calculada do
servidor. Ao alterar os limiares do talhão, a interface descartava as consultas
do dispositivo e da lista, mas não a da série. O desenho se corrigia sozinho,
porque as bandas são derivadas dos limiares novos; o texto não, porque vinha do
cache antigo. Um usuário de leitor de tela continuaria ouvindo a classificação
anterior enquanto o gráfico na mesma página já exibia a nova. *O defeito é
invisível para quem enxerga a tela*, e foi por isso que sobreviveu à revisão
visual --- as duas representações só se contradizem para quem depende da
segunda. Vale notar de onde veio a detecção: o defeito foi encontrado pelo
canal de acessibilidade, que a inspeção visual não exercita. O mesmo canal
havia denunciado, antes, as 76 interrupções inexistentes do defeito anterior,
porque é ele que enuncia em palavras aquilo que o desenho apenas sugere.

Sobre esse ponto, cabe uma precisão. A legenda visível do gráfico
deliberadamente *não* informa quantas interrupções houve, porque artefato de
alinhamento da grade de agregação e reporte efetivamente perdido são
indistinguíveis na série agregada, e contá-los afirmaria uma precisão que o
dado não tem. O resumo para leitor de tela, ao contrário, informa o número de
trechos, porque a navegação não visual precisa saber quantos elementos
percorrer. As duas escolhas são coerentes entre si e a assimetria é
intencional, mas ela significa que a interface *não* é uniformemente silenciosa
quanto à contagem --- e foi justamente o canal que conta que tornou o defeito
audível.

== Verificação

=== Suíte automatizada

A verificação do serviço de retaguarda é feita por 67 funções de teste, que se
desdobram em 87 casos executáveis quando os subtestes parametrizados são
contados individualmente. A distinção é registrada porque os dois números
descrevem a mesma suíte e divergem por um fator de organização do código: 67 é
o que se conta lendo os arquivos, 87 é o que a ferramenta reporta ao executar.
As funções distribuem-se por três dos oito pacotes do serviço, e são
executadas contra uma instância real do PostgreSQL, e não contra substitutos
em memória. A escolha decorre de o comportamento sob verificação depender de
propriedades do próprio banco de dados: a rejeição de leituras duplicadas pela
chave primária composta e o resultado das consultas de autorização por junção
não são reproduzíveis por um substituto sem reimplementá-lo, o que faria o
teste verificar a reimplementação em vez do sistema.

A camada de apresentação é verificada por 86 casos distribuídos em oito
arquivos, que cobrem a segmentação da série, a geometria da régua, a ordem de
verificação dos estados da leitura e a tradução de zona em aparência.

Os comportamentos críticos cobertos são a inversão de sinal na classificação
das faixas de atenção; as fronteiras exatas dos limiares, verificadas nos
valores iguais aos limiares e nos valores imediatamente adjacentes; o descarte
idempotente de leituras retransmitidas; o isolamento entre usuários, incluindo
a verificação de que um recurso fora do alcance responde 404 e não 403; a
invisibilidade de dispositivos sem vínculo com talhão; e a expiração de
sessões.

=== Verificação ponta a ponta do gerenciamento de nós

A suíte automatizada não substitui a execução do caminho completo. Os três
defeitos relatados na @sec-apresentacao passaram por ela sem falhar, porque um
teste automatizado verifica o sistema contra as suposições de quem o escreveu,
e todos os três decorriam de uma suposição errada a respeito do comportamento
do outro lado.

O caminho do RF10 foi, por isso, exercido integralmente contra o serviço em
execução, pela própria interface, e não por chamadas isoladas de API nem
contra o dublê de desenvolvimento. O resultado está na @tab-rf10.

#figure(
  caption: [Verificação ponta a ponta do gerenciamento de nós sensores (RF10)],
  text(size: 10pt)[#table(
    columns: (1fr, 1.35fr, 1.1fr),
    inset: (x: 5pt, y: 4.5pt),
    align: left + horizon,
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Passo*], [*Como foi feito*], [*Resultado*],
    ),
    [Cadastrar nó], [Formulário de cadastro], [Criado; token exibido uma única vez],
    [Editar descrição], [Painel de detalhe do nó], [Alterado, com o talhão preservado],
    [Mover para talhão sem concessão], [Alteração do vínculo de talhão], [Recusado como recurso não encontrado],
    [Enviar alteração vazia], [Painel de detalhe do nó], [Nenhuma requisição emitida],
    [Desativar o nó], [Painel de detalhe do nó], [Desativado],
    [Ingerir leitura com o nó desativado], [Envio de leitura com o token real emitido no cadastro], [Recusado por dispositivo inativo],
  )],
) <tab-rf10>
#fonte[Elaborado pelo autor (2026).]

A última linha é a que fecha o requisito. A confirmação exibida ao desativar um
nó promete, em português, que o servidor passará a recusar as leituras daquele
dispositivo. A recusa efetiva na ingestão, obtida com o token verdadeiro
emitido no cadastro, *transforma essa frase em fato verificado, em vez de
intenção de quem redigiu o texto da tela.* O mesmo ensaio estabelece, de
passagem, um segundo fato: a recusa foi por dispositivo inativo, e não por
token inválido, o que confirma que o valor mostrado uma única vez na tela é
efetivamente o valor que autentica.

A recusa da terceira linha é deliberadamente ambígua. Ela funde três situações
distintas --- nó inexistente, nó sem concessão e talhão sem concessão --- em
uma única resposta de recurso não encontrado, pela razão exposta na
@sec-autorizacao: distingui-las revelaria, a quem não tem acesso, a existência
de recursos de outros usuários.

=== Avisos que podem ser silenciados

Um padrão observado durante o desenvolvimento merece registro, por ter
ocorrido de forma independente em duas camadas distintas do sistema. Na
compilação do firmware, uma condição de configuração insegura era sinalizada
por diretiva `#warning`; verificou-se, porém, que a ferramenta de linha de
comando utilizada na compilação aceita a opção `-w`, que suprime a totalidade
dos avisos, de modo que a sinalização podia desaparecer sem qualquer ação
deliberada do desenvolvedor. A correção foi acrescentar uma diretiva
`#pragma message`, não afetada por essa supressão. No serviço de retaguarda,
uma configuração igualmente insegura --- cookie de sessão sem o atributo de
transporte seguro --- era registrada pelo mecanismo de log estruturado, cujo
manipulador descarta mensagens abaixo do nível configurado; um ajuste de nível
de log em produção silenciaria o alerta. A correção foi emitir o aviso
diretamente na saída de erro, fora do filtro de nível.

A lição comum é que um aviso que pode ser silenciado por configuração não é um
aviso, e a sua ocorrência independente em duas camadas construídas com
tecnologias distintas sugere que se trata de um padrão de projeto de
ferramentas, e não de acidente isolado. A verificação de que o alerta
efetivamente aparece é, portanto, parte da verificação do sistema, e não
detalhe de implementação.

== Lacunas metrológicas identificadas <sec-lacunas>

Além das correções de coluna de água e da incerteza do padrão de referência já
descritas na seção 1.3.3, a análise do arranjo experimental identificou quatro
lacunas que condicionam a interpretação de qualquer medida obtida.

A primeira é o limite de cavitação. A coluna de água no interior do
tensiômetro não suporta tensões arbitrariamente altas: a partir de certo
ponto, formam-se bolhas que rompem a continuidade hidráulica e invalidam a
leitura. A literatura estabelece o valor de 80 kPa como limite prático de
leitura máxima de operação do tensiômetro @azevedo1999, valor coerente com a
faixa de validação declarada por trabalhos correlatos, que verificam o
instrumento até $-80$ kPa @abdelmoneim2023. A consequência é que a faixa útil
efetiva do conjunto é menor que a faixa nominal do transdutor: dos $-100$ kPa
disponíveis eletronicamente, apenas cerca de quatro quintos correspondem a
medidas fisicamente válidas.

A segunda é o volume morto introduzido pela mangueira de acoplamento. O
transdutor não é acoplado diretamente à câmara de ar do tensiômetro, mas por
meio de um trecho de mangueira, cujo volume interno se soma ao volume da
câmara. Esse acréscimo tende a retardar a equalização de pressão e, portanto,
a resposta do conjunto a variações rápidas --- efeito que não foi quantificado
e que, diferentemente da constante de tempo do filtro elétrico, não pode ser
calculado a partir de dados de catálogo.

A terceira decorre de uma advertência explícita do fabricante do transdutor,
que registra a possibilidade de flutuação do sinal de saída sob incidência de
luz @cfsensor. A advertência é particularmente pertinente a este arranjo,
uma vez que o tubo do tensiômetro empregado é transparente. Não foram
realizados ensaios comparativos entre condições de iluminação, de modo que a
magnitude desse efeito no arranjo específico permanece desconhecida.

A quarta diz respeito à separação entre água e ar. O transdutor admite como
meio de medição apenas gases não corrosivos, não sendo estanque à água
@cfsensor. O acoplamento deve, portanto, garantir que o componente permaneça
em contato exclusivamente com a coluna de ar da câmara do tensiômetro, jamais
com a água do instrumento --- restrição que condicionou a geometria da
derivação física descrita a seguir.

== Derivação física do instrumento e retificação do diagnóstico

A derivação física é a conexão que acopla o transdutor ao tensiômetro sem
interromper a leitura do vacuômetro mecânico. Ela é pré-requisito da
calibração experimental, precisamente porque a comparação entre o protótipo e
o padrão de referência exige que ambos leiam a mesma câmara ao mesmo tempo.

A solução construída consiste na perfuração do septo de borracha no topo do
tensiômetro com uma agulha veterinária, acoplada à mangueira de silicone e
fixada por abraçadeira. O princípio é padrão em instrumentação --- a borracha
fecha ao redor da agulha ---, e a montagem é reversível: removida a agulha, o
septo veda novamente.

O primeiro teste dessa derivação constatou que o sistema não retinha vácuo. A
sucção aplicada alcançava o vacuômetro, o que descartava obstrução do caminho,
mas a pressão não se sustentava. *O diagnóstico registrado na ocasião atribuiu
o vazamento à derivação improvisada, e estava incorreto.*

A causa era outra: a cápsula cerâmica estava dessaturada. O instrumento havia
permanecido exposto ao ar durante semanas de testes, e uma cápsula cerâmica
seca deixa de funcionar como barreira hidráulica --- os poros que, saturados,
transmitem a tensão da água e bloqueiam a passagem de ar tornam-se, secos, um
caminho aberto para o ar entrar. O tensiômetro não estava vazando pela
derivação; estava vazando pela função que a cápsula deixara de exercer. Após
submersão prolongada da cápsula, com a ressaturação do material poroso, o
sistema passou a reter vácuo, e a derivação por septo perfurado mostrou-se
adequada.

O registro desta retificação tem valor metodológico independente do seu
desfecho favorável. A montagem improvisada era o componente novo do arranjo, e
por isso o suspeito natural; a cápsula era o componente comercial, e por isso
foi tratada como dado. A atribuição da falha ao elemento mais recente, e não
ao elemento cujo estado não havia sido verificado, adiou o diagnóstico
correto. Verificar a saturação da cápsula antes de qualquer ensaio hidráulico
passa a integrar o procedimento.

Com a retenção de vácuo restabelecida, a derivação deixa de ser o impedimento
à calibração experimental. Esta permanece não realizada, mas por não ter sido
conduzida, e não por estar bloqueada.

== Resultados de bancada e limites da interpretação <sec-bancada>

Esta seção separa deliberadamente o que foi observado do que ainda não pode
ser afirmado. Ela relata dois ensaios contínuos, retifica um resultado
anteriormente registrado e enumera, ao final, as afirmações que os dados
obtidos não sustentam.

=== Verificação funcional preliminar

Antes dos ensaios contínuos, o comportamento do conjunto foi verificado em
amostras curtas de bancada, com o transdutor alimentado e aberto à atmosfera.
Os resultados estão sintetizados na @tab-bancada.

#figure(
  caption: [Verificação funcional preliminar, com o transdutor aberto à atmosfera],
  text(size: 10pt)[#table(
    columns: (1.35fr, 1fr, 1.65fr),
    inset: (x: 5pt, y: 4.5pt),
    align: left + horizon,
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Grandeza observada*], [*Valor medido*], [*Interpretação*],
    ),
    [Tensão de saída em repouso, transdutor aberto à atmosfera],
    [4,524 V],
    [Corresponde a aproximadamente $0$ kPa; o valor previsto em catálogo é 4,5 V, e o desvio de 24 mV equivale a cerca de 0,6 kPa],
    [Dispersão entre amostras consecutivas],
    [±3,5 mV no pino],
    [Equivale a ±0,13 kPa após reconstrução pelo fator do divisor],
    [Resposta a sucção aplicada manualmente],
    [Qualitativa],
    [Sinal e sentido corretos: a aplicação de sucção reduz a tensão de saída, como previsto pela curva inversa],
  )],
) <tab-bancada>
#fonte[Elaborado pelo autor (2026).]

O desvio de 0,6 kPa entre a leitura em repouso e o valor de catálogo situa-se
confortavelmente dentro do erro máximo declarado para o próprio transdutor, de
±2% do fundo de escala, equivalente a ±2 kPa
@cfsensor, e é ainda compatível com a hipótese de que a alimentação real do
sensor seja ligeiramente inferior aos 5,00 V assumidos pelo firmware ---
hipótese plausível dado o comportamento ratiométrico do componente, e que a
calibração experimental absorverá.

A dispersão de ±3,5 mV estimada aqui é a única grandeza desta tabela que os
ensaios contínuos vieram a corrigir: medida sobre milhares de amostras, ela se
mostrou menor, como se relata adiante. Amostras curtas superestimam a
dispersão porque não distinguem o ruído de amostra a amostra da variação lenta
que se acumula ao longo de horas.

=== Condições dos dois ensaios

Foram realizados dois ensaios contínuos de longa duração, ambos com o
transdutor alimentado e aberto à atmosfera. O segundo repetiu deliberadamente
as condições do primeiro, e não o substitui: a comparação entre eles é parte
do resultado. As condições estão na @tab-condicoes.

#figure(
  caption: [Condições dos dois ensaios contínuos],
  text(size: 10pt)[#table(
    columns: (1.15fr, 1fr, 1fr),
    inset: (x: 5pt, y: 4.5pt),
    align: left + horizon,
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Condição*], [*Primeiro ensaio*], [*Segundo ensaio*],
    ),
    [Período (UTC)],
    [26/08 20:01 -- 27/08 12:48],
    [02/09 19:59 -- 03/09 09:35],
    [Duração],
    [16 h 47 min],
    [13 h 36 min],
    [Acoplamento do transdutor],
    [Aberto à atmosfera],
    [Aberto à atmosfera],
    [Condicionamento],
    [Divisor 10 kΩ/20 kΩ, fator 1,5; filtro de 100 nF],
    [Idêntico],
    [Alimentação],
    [USB, 5,00 V nominais assumidos, não medidos],
    [Idêntica],
    [Coeficientes de conversão],
    [Nominais de catálogo],
    [Nominais de catálogo],
    [Intervalo de amostragem],
    [60 s],
    [60 s],
    [Transporte],
    [HTTP sem TLS, servidor em rede local],
    [HTTP sem TLS, servidor em rede local],
    [Local da montagem],
    [Não registrado],
    [Cômodo com sinal de rede fraco na primeira hora; realocado em seguida],
  )],
) <tab-condicoes>
#fonte[Elaborado pelo autor (2026).]

Duas condições fogem ao arranjo previsto para o sistema em operação e devem
ser lidas com atenção. O transporte foi HTTP sem TLS contra um servidor em
rede local, e não HTTPS contra um servidor remoto como especificado no RF04:
as métricas de latência e de perda aqui reportadas não incluem, portanto, o
custo do estabelecimento da sessão segura nem a travessia da rede externa. E o
local da montagem no primeiro ensaio não foi registrado, o que impede afirmar
que as condições de rede dos dois ensaios eram equivalentes --- justamente a
variável que a subseção sobre confiabilidade identifica como determinante.

A alimentação merece registro explícito: o valor de 5,00 V é o nominal
assumido pelo firmware e gravado no campo `vdd_mv` de cada leitura, e não uma
medida. Qualquer desvio real da alimentação propaga-se para a tensão
reconstruída pelo comportamento ratiométrico do transdutor, e é uma das
hipóteses candidatas para o desvio observado em repouso.

=== Estabilidade do sinal

A @tab-estabilidade reúne as estatísticas do sinal nos dois ensaios.

#figure(
  caption: [Estabilidade do sinal nos dois ensaios contínuos],
  text(size: 10pt)[#table(
    columns: (1.2fr, 1fr, 1fr),
    inset: (x: 5pt, y: 4.5pt),
    align: (left, right, right),
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Métrica*], [*Primeiro ensaio*], [*Segundo ensaio*],
    ),
    [Amostras entregues], [1007], [784],
    [Média do sinal no pino], [3010,3 mV], [3012,24 mV],
    [Desvio padrão], [1,75 mV], [2,64 mV],
    [Mínimo], [3005 mV], [3004 mV],
    [Máximo], [3020 mV], [3020 mV],
    [Média em kPa], [0,384], [0,458],
    [Desvio padrão em kPa], [0,065], [0,099],
  )],
) <tab-estabilidade>
#fonte[Elaborado pelo autor (2026).]

Os desvios padrão, de 1,75 mV e 2,64 mV no pino, correspondem a
aproximadamente 0,066 kPa e 0,099 kPa após a reconstrução pelo fator do
divisor. Ambos ficam bem abaixo da resolução do vacuômetro mecânico tomado
como referência, e uma ordem de grandeza abaixo do erro máximo declarado para
o próprio transdutor. A dispersão do sinal, portanto, não é o fator limitante
da exatidão do conjunto.

A média em torno de 0,4 kPa com o transdutor aberto à atmosfera não é erro de
medição, mas o desvio dos coeficientes nominais de catálogo em relação ao
componente real. É exatamente a grandeza que a calibração experimental
existiria para absorver, e a sua persistência entre os dois ensaios ---
0,384 e 0,458 kPa --- é consistente com um desvio sistemático de coeficiente, e
não com ruído.

=== Confiabilidade da transmissão

A taxa de perda de pacotes exige duas leituras, e apresentá-la como um único
número seria enganoso nos dois sentidos possíveis.

No primeiro ensaio, 1009 leituras foram emitidas, 1008 chegaram ao serviço e
uma se perdeu, o que corresponde a 0,099%. No segundo, 811 leituras foram
emitidas e 784 chegaram: 27 perdas, ou 3,329% --- uma taxa mais de trinta
vezes maior, na mesma montagem e sob as mesmas condições nominais.

A distribuição das perdas explica a diferença. As 27 perdas do segundo ensaio
ocorreram todas entre 20:04 e 20:41 UTC, em seis rajadas, dentro da primeira
hora de operação. Essa hora concentrou 57 das 811 emissões e entregou 30
delas; todas as demais horas completas entregaram entre 59 e 60 leituras, sem
perda alguma. A causa foi identificada durante o ensaio: a montagem estava em
um cômodo com sinal de rede fraco. Após a realocação do conjunto, o
comportamento mudou de forma abrupta e permanente --- da leitura de número
1120 até a última, foram *772 emissões consecutivas sem nenhuma perda, ao
longo de 12 h 53 min de operação contínua*.

As duas taxas descrevem coisas diferentes. Os 3,329% descrevem o ensaio
inteiro, incluindo o período em que a condição de rede era inadequada, e são o
número honesto a reportar para aquele ensaio como um todo. A ausência de
perdas em 772 emissões descreve o regime permanente, depois de removida a
causa identificada.

A comparação que sustenta a interpretação é interna ao próprio segundo ensaio,
e é a mais controlada de que se dispõe: mesma montagem, mesmo firmware, mesmo
protocolo, mesma sessão de operação, e a única variável alterada foi a posição
física do conjunto. Antes da realocação, 27 perdas em 39 emissões; depois,
nenhuma em 772. *A perda observada é atributo da condição de rede, não do
protocolo de transmissão.*

O primeiro ensaio, com 0,099% de perda, corrobora essa leitura ao mostrar que
a mesma pilha de software opera com perda praticamente nula quando o enlace é
adequado. A corroboração é mais fraca do que parece, contudo, e a razão está
na @tab-condicoes: o local da montagem do primeiro ensaio não foi registrado,
de modo que a diferença entre as duas taxas totais não pode ser atribuída à
qualidade do enlace com o mesmo rigor da comparação interna. É por isso que o
argumento se apoia na transição observada dentro do segundo ensaio, e não no
contraste entre os dois.

Cabe registrar o que esse resultado não estabelece. Nenhum dos dois ensaios
submeteu o enlace a uma condição adversa controlada, com atenuação medida ou
interferência conhecida; a degradação do segundo ensaio foi encontrada, e não
provocada. O comportamento do nó sob perda prolongada de conectividade, e a
eficácia do mecanismo de retransmissão idempotente sob essa condição,
permanecem não caracterizados.

=== Latência e o desvio entre relógios

A @tab-latencia apresenta a diferença entre os dois instantes registrados em
cada leitura: o carimbo de medição, gravado pelo nó, e o de recepção, gravado
pelo serviço.

#figure(
  caption: [Diferença entre os instantes de medição e de recepção],
  text(size: 10pt)[#table(
    columns: (1.2fr, 1fr, 1fr),
    inset: (x: 5pt, y: 4.5pt),
    align: (left, right, right),
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Métrica*], [*Primeiro ensaio*], [*Segundo ensaio*],
    ),
    [Média], [0,592 s], [0,553 s],
    [Mediana], [---], [0,513 s],
    [Mínimo], [$-"0,123"$ s], [$-"0,131"$ s],
    [Máximo], [4,464 s], [7,733 s],
    [Leituras com valor negativo], [1], [27],
  )],
) <tab-latencia>
#fonte[Elaborado pelo autor (2026).]

O mínimo negativo, reproduzido nos dois ensaios com magnitude praticamente
idêntica, é um achado e não um erro de cálculo: uma leitura não pode ser
recebida antes de ser medida. O que a grandeza mede, na verdade, é a soma da
latência real de transmissão com o desvio entre dois relógios distintos --- o
do ESP32, sincronizado por NTP, e o do servidor. Quando a latência real é
menor que esse desvio, a soma fica negativa.

O valor absoluto do mínimo, de aproximadamente 0,13 s nos dois ensaios,
estabelece uma cota inferior para o desvio entre os relógios. Não é possível,
com os dados disponíveis, separar as duas parcelas: a métrica reportada é de
latência aparente, e assim deve ser lida. Para o intervalo de amostragem
previsto em campo, da ordem de dezenas de minutos, um desvio dessa magnitude é
irrelevante; a limitação é de método, não de aplicação.

=== Retificação: a deriva não se confirmou

O registro de bancada do primeiro ensaio descreveu a variação do sinal ao
longo da noite como deriva, com amplitude de 3,76 mV --- cerca de 0,14 kPa ---
entre a média horária mínima e a máxima. *O segundo ensaio não reproduziu esse
comportamento, e a afirmação é aqui retificada.*

A @tab-deriva compara as duas séries de médias horárias após o período de
acomodação inicial descrito na subseção seguinte.

#figure(
  caption: [Tendência das médias horárias após a acomodação inicial],
  text(size: 10pt)[#table(
    columns: (1.3fr, 1fr, 1fr),
    inset: (x: 5pt, y: 4.5pt),
    align: (left, right, right),
    stroke: (x, y) => if y == 0 { (bottom: 0.75pt) } else { (bottom: 0.5pt) },
    table.header(
      [*Métrica*], [*Primeiro ensaio*], [*Segundo ensaio*],
    ),
    [Horas consideradas], [14], [12],
    [Amplitude], [3,76 mV], [3,05 mV],
    [Inclinação da reta ajustada], [$+"0,222"$ mV/h], [$-"0,065"$ mV/h],
    [Coeficiente de determinação], [0,86], [0,06],
  )],
) <tab-deriva>
#fonte[Elaborado pelo autor (2026).]

A amplitude é da mesma ordem nos dois ensaios, mas a forma não é. No primeiro,
as médias horárias sobem de maneira quase monotônica, e uma reta explica 86%
da variação --- o que motivou a leitura original como deriva. No segundo, o
sinal oscila entre 3010,60 mV e 3013,65 mV sem direção definida: a reta
ajustada tem inclinação de sinal oposto e explica 6% da variação, ou seja,
praticamente nada.

Com duas observações, uma apresentando tendência e a outra não, a deriva não
pode ser afirmada como sistemática. O que os dois ensaios sustentam em comum é
mais fraco e mais preciso: *existe uma variação de baixa frequência, de
amplitude em torno de 3 mV ao longo de uma dezena de horas, cuja causa não foi
determinada e cujo sinal não é reprodutível entre corridas.* A hipótese de
ciclo térmico acompanhando a temperatura ambiente, formulada e descartada no
primeiro ensaio pela posição do mínimo no horário errado, continua descartada;
nenhuma hipótese alternativa foi testada. A magnitude permanece pequena frente
à especificação do transdutor, mas o fenômeno deve ser declarado, e não
nomeado além do que a evidência permite.

Registre-se também a assimetria de método entre as duas linhas da tabela: a
regressão do primeiro ensaio foi calculada sobre médias horárias, únicos dados
sobreviventes daquele ensaio, e a do segundo foi calculada sobre as mesmas
médias horárias justamente para que a comparação fosse legítima. Ajustada
sobre as amostras individuais, que estão disponíveis apenas para o segundo
ensaio, a inclinação é de $-"0,082"$ mV/h com coeficiente de determinação de
0,011 --- a conclusão não muda, mas os dois números não seriam comparáveis
entre si, porque a média horária remove o ruído intra-hora e infla
artificialmente o ajuste.

=== O que se repetiu: acomodação inicial

Um comportamento, esse sim, apareceu nos dois ensaios. Ambos começam com o
sinal mais alto e decrescem até estabilizar, em um intervalo de duas a três
horas a partir da energização.

No segundo ensaio, em que a série completa está disponível, as médias horárias
partem de 3020,00 mV, passam por 3015,23 mV e 3012,73 mV, e chegam a
3011,54 mV na quarta hora, quando a queda cessa: uma acomodação de cerca de
8,5 mV. No primeiro ensaio, o mesmo movimento aparece com amplitude menor, de
3011,32 mV para 3008,97 mV ao longo das três primeiras horas.

A acomodação de transdutores piezorresistivos após a energização é fenômeno
conhecido, atribuído ao equilíbrio térmico do próprio elemento sensor e da
eletrônica de condicionamento.
#pendente[REF PENDENTE: fonte que documente o transitório de aquecimento em
transdutores piezorresistivos após energização --- o fenômeno é conhecido em
instrumentação, mas nenhuma referência foi confirmada para citá-lo.]

Independentemente da causa, a consequência metodológica é imediata e vale como
recomendação para os ensaios de calibração ainda por realizar: *as primeiras
horas de qualquer ensaio devem ser descartadas ou registradas em separado.*
Incluí-las na estatística agregada contamina a média com um transitório que
não descreve o regime de operação --- no segundo ensaio, a acomodação sozinha
responde por toda a amplitude entre o máximo e o mínimo das médias horárias.

=== Reprodutibilidade entre corridas

Os dois ensaios mediram a mesma grandeza física --- a pressão atmosférica,
com o transdutor aberto --- com a mesma montagem, em dias diferentes. As
médias obtidas foram 3010,3 mV e 3012,24 mV, uma diferença de *1,94 mV, ou
aproximadamente 0,073 kPa*.

Essa diferença é uma métrica distinta do desvio padrão de cada ensaio, e não
deve ser confundida com ele. O desvio padrão descreve a dispersão interna de
uma corrida, com a montagem intacta e a alimentação ligada; a diferença entre
médias descreve o que muda quando a corrida é refeita. Ela absorve o que o
desvio padrão isolado não vê: variação da alimentação entre as sessões,
condições ambientais distintas, o estado térmico do conjunto e a própria
pressão atmosférica real, que não é constante entre dois dias.

A consequência deve ser declarada: *a incerteza real do sistema é maior que o
desvio padrão medido dentro de um único ensaio.* Isso é esperado e não
constitui defeito; o que seria defeito é apresentar o desvio padrão intra-ensaio
como se fosse a incerteza do instrumento. Duas corridas, contudo, não permitem
estimar um intervalo de confiança para a reprodutibilidade --- apenas
constatar que ela é da ordem de 2 mV, e que qualquer calibração futura deve
ser avaliada contra essa escala, e não contra a dispersão interna.

=== Perda dos dados do primeiro ensaio

Os dados brutos do primeiro ensaio foram perdidos. O volume do banco de dados
foi removido acidentalmente, e a cópia de segurança não era válida: o arquivo
de exportação existia, com o nome esperado e a data esperada, mas com zero
bytes. A causa é prosaica e vale como registro --- o redirecionamento de saída
do interpretador de comandos cria o arquivo antes de executar o comando, de
modo que um comando que falha deixa para trás um arquivo vazio, e não a
ausência de arquivo que sinalizaria a falha.

A nota metodológica que decorre disso é que *a verificação da integridade da
cópia de segurança, e não apenas da sua existência, é parte do procedimento
experimental.* Conferir que o arquivo tem tamanho plausível e que o número de
registros corresponde ao esperado são operações de segundos, e são a diferença
entre um dado recuperável e um dado perdido.

A consequência epistêmica é assimétrica entre os dois ensaios, e a distinção
percorre toda esta seção. A série do segundo ensaio está preservada no banco
de dados: cada número aqui apresentado pode ser recalculado, e uma análise
diferente da que se fez pode ser conduzida sobre os mesmos dados. Os números
do primeiro ensaio, ao contrário, sobrevivem apenas no registro de bancada
redigido na ocasião --- são fonte secundária, não reprocessável, e nenhuma
verificação independente pode ser feita sobre eles. É por isso que a
retificação da deriva, na @tab-deriva, precisou usar as médias horárias
transcritas naquele registro: elas são tudo o que restou daquele ensaio.

=== Limites da interpretação

As afirmações a seguir não são autorizadas pelos dados obtidos.

*Não houve calibração experimental.* Os coeficientes empregados na conversão
são os valores nominais de catálogo, não valores ajustados por comparação
contra um padrão. Em consequência, não é possível apresentar coeficiente de
determinação, erro quadrático médio ou erro percentual do protótipo: essas
métricas pressupõem um conjunto de pares de medidas simultâneas entre o
protótipo e um instrumento de referência, que não foi levantado. O desvio
sistemático em torno de 0,4 kPa observado nos dois ensaios é precisamente a
grandeza que essa calibração absorveria.

*Não houve validação em solo.* Todos os ensaios foram conduzidos em bancada,
com o transdutor aberto à atmosfera ou submetido a sucção manual. O
comportamento do conjunto instalado em solo, sujeito à dinâmica real de
secagem e umedecimento e às correções descritas na @sec-lacunas, permanece não
verificado. Um transdutor aberto à atmosfera exercita a cadeia de aquisição,
de conversão e de transmissão; não exercita o instrumento.

*A variação de baixa frequência não está caracterizada.* Como exposto acima,
os dois ensaios divergem quanto à existência de tendência, e nenhuma causa foi
estabelecida. Afirmar deriva sistemática, ou negá-la, excede o que duas
observações sustentam.

*O enlace não foi submetido a condição adversa controlada.* A degradação
observada no início do segundo ensaio foi encontrada e corrigida, não
provocada e medida. O comportamento do sistema sob perda prolongada de
conectividade não foi caracterizado.

#pendente[FOTO PENDENTE: tensiômetro com o vacuômetro mecânico acoplado,
utilizado como padrão de referência nos ensaios de calibração previstos.]
