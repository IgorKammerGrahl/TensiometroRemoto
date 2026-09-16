#import "../abnt.typ": pendente

= Introdução

O setor agrícola vem passando por profundas transformações tecnológicas para
atender à crescente demanda global por alimentos. Essa evolução consolidou o
conceito de Agricultura de Precisão (_Precision Agriculture_), uma estratégia
de gestão centrada na observação, medição e resposta à variabilidade espacial
e temporal das lavouras, com o objetivo de otimizar a aplicação de insumos e
reduzir impactos ambientais @gebbers2010. Com os avanços recentes, esse
conceito expandiu-se para a Agricultura Digital ou Inteligente (_Smart
Farming_), que emprega a captura e o processamento massivo de dados para
embasar decisões agronômicas @wolfert2017.

O pilar central dessa revolução é a Internet das Coisas (IoT). Sua aplicação
na agricultura permite a implantação de redes de sensores, atuadores e
comunicação sem fio diretamente no campo, viabilizando o monitoramento em
tempo real de variáveis ambientais e a automação de processos. Essa
integração transforma o paradigma de uma agricultura estritamente empírica
para um manejo de precisão orientado a dados @elijah2018.

Apesar desses avanços, a agricultura contemporânea ainda enfrenta o desafio
de garantir a segurança alimentar global em um cenário de instabilidade
climática e escassez crescente de recursos hídricos. Atualmente, a
agricultura irrigada é responsável por aproximadamente 70% a 80% do consumo
de toda a água doce disponível no planeta @ghazi2025 @lakhiar2024. A
sustentabilidade desse modelo, no entanto, é severamente ameaçada pela
ineficiência do uso da água, o que evidencia a necessidade de transição para
práticas tecnológicas mais eficientes e sustentáveis.

Nesse contexto, os sistemas de irrigação inteligente (_Smart Irrigation
Systems_) unem os preceitos da IoT à necessidade hídrica das culturas,
surgindo como soluções capazes de fornecer água estritamente sob demanda,
sem a obrigatoriedade de intervenção humana constante @garcia2020 @sarr2026.
Para que essa automação seja fisiologicamente assertiva para a planta, o
monitoramento da umidade do solo figura como o parâmetro mais crítico. O uso
de tensiômetros destaca-se nesse cenário pois, ao contrário de sensores
volumétricos comuns, eles medem o potencial matricial da água no solo — a
força exata que o sistema radicular precisa exercer para absorver a água —,
fornecendo um dado fisiológico direto sobre o nível de estresse hídrico da
cultura @abdelmoneim2023.

== Problematização

O manejo da irrigação tradicional baseia-se, na maioria das vezes, em
cronogramas fixos, estimativas climáticas genéricas ou na simples observação
visual do agricultor. Essa abordagem empírica frequentemente resulta em dois
cenários prejudiciais: a sobre-irrigação, que causa desperdício de água,
lixiviação de nutrientes, aumento dos custos de energia e proliferação de
doenças fúngicas; ou o subfornecimento, que leva a cultura ao estresse
hídrico e à redução drástica da produtividade @purnama2024.

Embora o tensiômetro seja o instrumento padrão-ouro em termos de resposta
fisiológica para o agendamento da irrigação, sua versão mecânica tradicional
exige leituras manuais constantes e manutenção frequente. Essa exigência
operacional inviabiliza o seu uso em larga escala na agricultura moderna sem
a devida automação tecnológica @abdelmoneim2023. Além disso, a ausência de
sistemas integrados e de baixo custo que conectem dados de tensiometria em
tempo real a plataformas de decisão acessíveis afasta, especialmente, os
pequenos e médios produtores das vantagens da irrigação de precisão.

Diante desse cenário, formula-se a seguinte pergunta de pesquisa: de que
maneira é possível desenvolver um nó sensor de telemetria, fundamentado em
Internet das Coisas (IoT) e tensiometria eletrônica, que realize o
monitoramento remoto e contínuo da tensão da água no solo e disponibilize
esses dados em um aplicativo móvel para auxiliar na tomada de decisão
agrícola?

=== Delimitação do escopo

O projeto teve como escopo o desenvolvimento de um protótipo de monitoramento
e telemetria, contemplando a integração de hardware e software. No aspecto de
hardware, foi utilizado um microcontrolador com conectividade sem fio (placa
ESP32 DevKit) integrado ao transdutor de pressão analógico
XGZP6847A100KPGN, acoplado, por meio de uma mangueira de silicone, à câmara de ar
de um tensiômetro analógico de campo. No escopo de software, foram
implementadas a leitura elétrica do transdutor, a sua conversão matemática
para unidades de tensão (kPa), a transmissão de dados para um servidor em
nuvem e a visualização em um aplicativo móvel (ou painel de controle
digital).

Foge ao escopo deste trabalho a automação do acionamento hídrico físico
(acionamento de relés, eletroválvulas ou bombas de água, caracterizando uma
malha fechada), limitando-se o sistema a atuar como um provedor de dados
vitais para o manejo do produtor. Também não compõem o escopo a implementação
em uma lavoura comercial de grande extensão ou a análise agronômica de
produtividade de longo prazo em uma cultura específica. A validação
restringiu-se a um ambiente de bancada --- sem ensaio em estufa ou em solo,
conforme registrado entre as limitações do @cap-prototipo ---, para atestar a
funcionalidade da conversão analógica e da comunicação IoT do protótipo.

=== Justificativa

O presente trabalho justifica-se pela urgente necessidade de otimização dos
recursos hídricos na agricultura, interligando contribuições científicas,
tecnológicas, ambientais e sociais em uma única proposta. Sob a ótica
científica, o estudo avança na pesquisa aplicada de redes de sensores sem fio
(WSN) ao explorar a integração de medições baseadas no potencial matricial do
solo, utilizando a tensiometria eletrônica. Essa abordagem visa superar uma
lacuna metodológica comum em projetos acadêmicos de Internet das Coisas
(IoT), que frequentemente recorrem a sensores de umidade volumétrica
genéricos e de baixa precisão em relação ao real estresse hídrico da planta.

Para viabilizar essa precisão científica no campo, a dimensão tecnológica do
projeto fomenta a democratização da Agricultura 4.0. Ao empregar componentes
de prateleira de baixo custo e arquitetura aberta — como microcontroladores
com conectividade nativa e transdutores de pressão convencionais —,
demonstra-se a viabilidade de construir sistemas de agricultura de precisão
escaláveis e acessíveis. Essa arquitetura de hardware é complementada por uma
interface de monitoramento móvel, desenvolvida com foco na usabilidade e na
experiência do usuário, desmistificando o uso de tecnologias complexas no
meio rural.

Consequentemente, essa acessibilidade tecnológica reflete em impactos diretos
nas esferas ambiental e social. A literatura científica aponta que a
implementação de sistemas de irrigação inteligentes baseados em sensores pode
reduzir o consumo de água entre 15% e 40% @ghazi2025, além de otimizar
significativamente o uso de energia elétrica nas propriedades rurais
@correaquiroz2025. Dessa forma, o projeto promove ativamente a
sustentabilidade ambiental ao mitigar o desperdício de recursos críticos, ao
mesmo tempo em que cumpre um papel social ao empoderar o pequeno e médio
produtor com ferramentas de automação que reduzem a carga de trabalho manual
e aumentam a eficiência operacional de suas lavouras.

== Objetivos

=== Objetivo geral

Desenvolver um protótipo IoT de telemetria para o monitoramento da tensão da
água no solo, integrando a tensiometria eletrônica a um microcontrolador
ESP32 e a um aplicativo móvel para a visualização remota dos dados, visando o
suporte à decisão no manejo da irrigação.

=== Objetivos específicos

Para o atingimento do objetivo geral, foram traçados os seguintes objetivos
específicos:

+ Estudar os fundamentos teóricos da tensiometria, da instrumentação
  eletrônica e dos protocolos de comunicação IoT aplicados à agricultura de
  precisão;
+ Projetar e montar o nó sensor de hardware, acoplando um sensor de pressão
  eletrônico a um tensiômetro analógico e conectando-o ao microcontrolador
  ESP32;
+ Desenvolver o firmware embarcado para a leitura do sinal analógico do
  sensor e sua devida conversão matemática para o potencial matricial de água
  no solo (kPa);
+ Implementar a infraestrutura de comunicação em nuvem e desenvolver o
  aplicativo de monitoramento para exibir o histórico hídrico do solo em
  tempo real;
+ Avaliar o funcionamento operacional do sistema prototipado, mensurando a
  precisão da conversão dos sinais elétricos em relação ao vácuo gerado e a
  estabilidade da transmissão IoT.

== Procedimentos metodológicos

A metodologia deste trabalho caracteriza-se como uma pesquisa aplicada e de
natureza experimental, estruturada em três etapas principais que delineiam a
construção e a validação do sistema proposto: (a) especificação e montagem do
hardware; (b) desenvolvimento de software e integração em nuvem; e (c)
validação experimental e coleta de dados.

As duas primeiras etapas foram executadas integralmente, e a terceira, apenas
em parte. O nó sensor foi montado em matriz de contatos, o firmware do ESP32
foi escrito e a cadeia de telemetria, o serviço de retaguarda e a interface do
produtor foram construídos e verificados. Da validação experimental foram
realizados os ensaios funcionais e de confiabilidade da telemetria, em
bancada, com o transdutor aberto à atmosfera; permanecem por fazer a
calibração contra o vacuômetro mecânico e os ensaios de secagem em solo. O
@cap-prototipo relata o que foi construído e delimita o que os ensaios
executados autorizam afirmar.

=== Especificação e montagem do hardware

A primeira etapa consistiu na seleção e integração dos componentes físicos do
nó sensor. O microcontrolador escolhido para gerenciar a aquisição e
transmissão dos dados foi a placa ESP32 DevKit, justificada por seu baixo
custo, alto poder de processamento e conectividade Wi-Fi nativa, essenciais
para aplicações IoT @correaquiroz2025.

O monitoramento do solo não é volumétrico, mas baseado na sucção (vácuo)
gerada pela secagem da terra ao redor da cápsula cerâmica de um tensiômetro
analógico. Para a automação dessa leitura, a câmara de ar do tensiômetro foi
conectada, através de uma mangueira de silicone, ao transdutor de pressão
XGZP6847A100KPGN.

A designação XGZP6847A não identifica um componente único, mas uma família de
transdutores cujo sufixo do código de encomenda define a faixa e o tipo de
pressão medida: o sufixo `G` corresponde à pressão manométrica positiva, `GN`
à pressão negativa (vácuo) e `GPN` à faixa bidirecional @cfsensor. A variante
adotada neste trabalho, `100KPGN`, opera na faixa de $-100$ a $0$ kPa, o que
é coerente com o princípio de operação do tensiômetro, instrumento que mede
exclusivamente por sucção. Com alimentação de 5 V, o componente converte a
variação mecânica do vácuo interno em um sinal elétrico na faixa de 0,5 V a
4,5 V, em correspondência inversa: 0,5 V equivale a $-100$ kPa e 4,5 V
equivale a $0$ kPa @cfsensor.

O sinal foi direcionado a uma das portas analógicas do ESP32
@abdelmoneim2023 por meio de um divisor resistivo, necessário para
compatibilizar a saída de até 4,5 V do transdutor com a tensão máxima
tolerada pelo conversor analógico-digital (ADC) do microcontrolador, que
opera em 3,3 V.

=== Desenvolvimento de software e integração IoT

A segunda etapa abrangeu a programação do firmware e a configuração da
arquitetura de telemetria. O firmware do ESP32 foi desenvolvido em linguagem
C/C++ utilizando a plataforma Arduino IDE, que oferece vasto suporte a
bibliotecas para manipulação de sensores analógicos e módulos de rede.

O algoritmo embarcado foi projetado para efetuar a leitura contínua do pino
ADC do microcontrolador e aplicar a fórmula de conversão matemática
necessária para transformar a faixa de tensão obtida (Volts) em uma unidade
de pressão de sucção (kPa), refletindo o exato nível de esforço hídrico da
planta.

Paralelamente, foi configurada a comunicação IoT, enviando esses valores
convertidos via conexão Wi-Fi para um servidor. A interface do produtor foi
desenvolvida como aplicação web progressiva, fornecendo uma interface gráfica
interativa que permite ao usuário a visualização do histórico e das tendências
de variação da tensão da água no solo, sem a necessidade de deslocamento
físico até a lavoura.

=== Validação experimental e análise de dados

A etapa final consistiu na validação funcional do protótipo de monitoramento
em ambiente de bancada, e foi cumprida apenas em parte. As duas métricas
operacionais previstas tiveram destinos distintos:

- *Precisão da calibração analógica:* avaliação da correspondência entre o
  sinal de tensão (V) lido pelo ESP32 a partir do sensor XGZP6847A e a
  leitura visual apontada no vacuômetro mecânico acoplado ao sistema. Esse
  ensaio não foi realizado. Os coeficientes de conversão empregados
  permanecem os valores nominais de catálogo, e nenhuma métrica de ajuste
  pode ser apresentada.
- *Confiabilidade da telemetria IoT:* monitoramento da estabilidade e
  periodicidade do envio de pacotes de dados do microcontrolador para o
  serviço de retaguarda, verificando perdas de conexão ou latências na
  atualização dos dados na interface gráfica do usuário. Essa métrica foi
  medida em dois ensaios contínuos, relatados no @cap-prototipo.

A submissão do sistema a dinâmicas de secagem do solo, prevista para gerar
diferentes níveis de tensão na cápsula do tensiômetro, permanece por realizar.
Os ensaios executados mantiveram o transdutor aberto à atmosfera, condição que
exercita a cadeia de aquisição e de transmissão, mas não a resposta do
instrumento ao solo.

A comparação entre a leitura eletrônica e a leitura visual exige duas
correções que não são evidentes no arranjo experimental. A primeira decorre
da coluna de água. A cápsula cerâmica situa-se abaixo do manômetro, e o peso
da água contida no tubo exerce pressão hidrostática sobre o ponto de medição:
no nível da cápsula a pressão é maior do que no topo do instrumento, de modo
que o manômetro indica um valor mais severo do que a tensão efetivamente
imposta pelo solo. Na convenção de sinal adotada neste trabalho, em que a
tensão da água no solo é uma grandeza negativa, a correção é aditiva:

$ psi_"solo" = "leitura"_"manômetro" + "0,098" dot h $

com $h$ em centímetros e resultado em kPa. Para um instrumento de 60 cm, o
termo de correção vale $+"5,88"$ kPa: um manômetro indicando $-40$ kPa
corresponde a uma tensão real de $-"34,1"$ kPa, ou seja, o solo está menos seco
do que a leitura no topo sugere. A magnitude da correção é comparável à
própria resolução pretendida para o instrumento e, portanto, não é
desprezível.

A fonte consultada apresenta a mesma relação na forma $T_"as" = L - "0,098" h$
@azevedo1999, em que tanto a leitura $L$ quanto a tensão resultante são
expressas como magnitudes positivas de sucção. As duas formulações são
equivalentes e diferem apenas na convenção de sinal. Como todo o restante
deste documento --- faixa do transdutor, limiares de atenção e leituras de
bancada --- adota a representação negativa, a forma aditiva é a que deve ser
aplicada. Empregar a forma subtrativa sobre uma leitura já negativa deslocaria
o resultado do exemplo acima para $-"45,9"$ kPa, um erro de 11,8 kPa na direção
oposta à correta.

A segunda correção diz respeito à incerteza do próprio padrão de referência.
O vacuômetro mecânico acoplado ao tensiômetro é um instrumento de classe B
segundo a ABNT NBR 14105-1 @abnt14105, e sua tolerância limita a exatidão máxima que se pode
atribuir ao protótipo, uma vez que nenhuma calibração pode ser mais exata que
o padrão contra o qual é comparada. A validação experimental não estabelece,
portanto, a exatidão absoluta do transdutor, mas a sua concordância com um
padrão de exatidão conhecida e limitada.
#pendente[REF PENDENTE: valor numérico da tolerância admitida para a classe B
na ABNT NBR 14105-1 --- a norma é paga e o número não pôde ser confirmado.]

Esses procedimentos estabeleceram a viabilidade técnica da cadeia de aquisição
e de transmissão de um nó sensor de baixo custo como ferramenta de suporte
computacional à irrigação de precisão. A exatidão da grandeza medida, que
depende da calibração contra o padrão de referência, permanece fora do que os
ensaios executados autorizam afirmar.
