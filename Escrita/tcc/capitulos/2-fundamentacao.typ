= Fundamentação teórica

Este capítulo apresenta a base conceitual e tecnológica necessária para a
compreensão e o desenvolvimento deste trabalho de pesquisa. Para que a
integração de um protótipo de telemetria agrícola seja viável e eficiente, é
imprescindível compreender as tecnologias subjacentes e o contexto agronômico
no qual o sistema será inserido. Inicialmente, discute-se a evolução da
Agricultura de Precisão e da Internet das Coisas (IoT), que fornecem a
infraestrutura de comunicação do projeto, e os sistemas de irrigação
inteligente. Em seguida, apresentam-se o estado de energia da água no solo,
expresso pelo potencial matricial, e o tensiômetro, instrumento que o mede,
com os limites físicos e os cuidados hidráulicos de que a medição depende.
Abordam-se então a transdução eletrônica da pressão e os conceitos de
metrologia com que a medição é avaliada. Por fim, delineiam-se os componentes
de hardware e software que fundamentam a arquitetura de monitoramento
proposta.

== Agricultura de precisão e Internet das Coisas (IoT)

Historicamente, o manejo agrícola baseava-se em observações empíricas e na
aplicação uniforme de insumos em vastas extensões de terra,
independentemente da variabilidade local. Para mitigar o desperdício gerado
por essa abordagem, consolidou-se a Agricultura de Precisão, definida como
uma estratégia de gestão que utiliza tecnologias de observação para responder
à variabilidade espacial e temporal das lavouras, otimizando o uso de
recursos e minimizando impactos ambientais @gebbers2010.

Com a evolução das tecnologias de comunicação, a Agricultura de Precisão
expandiu-se para o paradigma da Agricultura Digital ou _Smart Farming_.
Segundo #cite(<wolfert2017>, form: "prose"), esse novo modelo é caracterizado
pela capacidade de capturar dados massivos em tempo real, analisá-los em
nuvem e retornar decisões acionáveis para o campo.

O motor tecnológico que viabiliza essa transição é a Internet das Coisas
(IoT -- _Internet of Things_). A IoT consiste em uma rede de objetos físicos
incorporados com sensores, software e conectividade de rede, permitindo a
coleta e a troca contínua de dados @elijah2018. Na agricultura, a implantação
de nós sensores IoT permite a substituição do trabalho manual de
monitoramento climático e de solo por redes sem fio (_Wireless Sensor
Networks_ -- WSN), promovendo uma automação inteligente em que as decisões de
manejo são embasadas em dados estatísticos e contínuos.

== Sistemas de monitoramento e irrigação inteligente

O setor agrícola é, globalmente, o maior consumidor de água doce, sendo
responsável por cerca de 70% do uso total desse recurso @lakhiar2024. Diante
da escassez hídrica impulsionada pelas mudanças climáticas, os métodos de
manejo convencionais provam-se insustentáveis. O manejo inadequado não apenas
desperdiça água: a irrigação em excesso aumenta as perdas de nutrientes por
lixiviação e reduz a eficiência no uso de fertilizantes @ghazi2025.

Para combater essa ineficiência, surgem os Sistemas de Irrigação Inteligente
(_Smart Irrigation Systems_). No escopo do monitoramento e da telemetria
contínua (sistemas de malha aberta ou de suporte à decisão), esses sistemas
utilizam a infraestrutura IoT para processar variáveis ambientais em tempo
real @garcia2020. Em vez de atuar cegamente sobre o campo, os nós sensores
captam as condições exatas do solo e transmitem essas métricas para
plataformas digitais. Essa abordagem fornece ao produtor rural a inteligência
informacional necessária para realizar o manejo hídrico no momento exato e na
quantidade estritamente requerida pela cultura, elevando significativamente a
eficiência do uso da água @sarr2026.

== A água no solo e o potencial matricial <sec-agua-solo>

A retenção de água no solo expressa a energia de ligação da água ao solo e
decorre da força gravitacional, das forças capilares e das propriedades de
superfície das partículas minerais e orgânicas, capazes de adsorver água
@teixeira2017. É essa energia, e não apenas a quantidade de água presente, que
condiciona a facilidade com que as raízes extraem a água: um solo pode conter
bastante água e, ainda assim, retê-la com força suficiente para dificultar a
extração, enquanto solos de textura mais arenosa armazenam menos água, mas a
liberam mais facilmente para as plantas @marouelli2008. A grandeza que exprime
essa retenção é o potencial matricial da água no solo, designado também como
tensão da água no solo ou tensão matricial quando expresso em módulo
@azevedo1999 @marouelli2008.

Por se tratar de energia por unidade de volume, a grandeza tem a dimensão de
uma pressão, e as unidades encontradas nos instrumentos são variadas:
1 kPa equivale a 1 cbar, a 7,5 mmHg e a cerca de 10,2 cmH#sub[2]O
@marouelli2008 @azevedo1999. As fontes agronômicas consultadas expressam a
tensão como magnitude positiva de sucção; este trabalho a representa com
sinal negativo, convenção cujas consequências para a correção de coluna de
água são discutidas na @sec-validacao. Nesta seção e na seguinte, os valores
são reproduzidos na forma em que as fontes os apresentam.

A relação entre o conteúdo de água e o potencial é descrita pela curva
característica de retenção de água do solo, que varia de um solo para outro e
é determinada em laboratório a partir de pares de potencial e umidade,
usualmente obtidos em potenciais entre 0 e 1.500 kPa @teixeira2017
@marouelli2008. Dois pontos dessa curva recebem denominação própria: a
umidade no potencial de 1.500 kPa é o ponto de murcha permanente (PMP), e a
umidade no potencial de 10 kPa ou de 33 kPa, a capacidade de campo (CC); a
diferença entre os dois é a água disponível (AD) para as plantas
@teixeira2017. Como o tensiômetro mede o potencial, e não a umidade, é a curva
de retenção que permite estimar o conteúdo de água a partir de uma leitura,
quando essa estimativa é necessária @marouelli2008 @azevedo1999.

Para o manejo da irrigação, a leitura é interpretada por faixas. Como regra
geral, tensões de 0 a 10 kPa indicam solo próximo à saturação, com perda de
água por drenagem profunda e deficiência de aeração para as raízes; de 10 a
20 kPa, excelente condição de umidade; de 20 a 40 kPa, boa condição de
umidade; de 40 a 70 kPa, condição de umidade limitada; e, acima de 70 kPa,
baixa disponibilidade de água @marouelli2008. O momento de irrigar, contudo,
não é dado por um valor universal. A tensão-limite recomendada varia com a
cultura, com o sistema de irrigação e com a textura do solo @marouelli2008, e
as próprias recomendações para uma mesma cultura divergem entre os autores
que a estudaram @azevedo1999. Essa variabilidade é a razão de as faixas de
atenção do sistema proposto serem configuráveis por talhão, e não fixadas no
código (@sec-faixas).

== Tensiometria <sec-tensiometria>

Para que um sistema de suporte à irrigação atue com precisão, a métrica de
controle deve refletir adequadamente a necessidade hídrica da planta. Grande
parte das implementações de baixo custo utiliza sensores de umidade
volumétrica resistivos ou capacitivos genéricos, que mensuram apenas a
porcentagem de água presente em um dado volume de terra. Essa abordagem
apresenta limitações, pois a presença de água no solo não garante que ela
esteja prontamente disponível para as raízes: a energia com que a água fica
retida, e portanto o esforço necessário para extraí-la, depende também da
textura, da estrutura e do teor de matéria orgânica do solo @abdelmoneim2023.
O tensiômetro, ao contrário, mede diretamente essa energia de retenção
@azevedo1999.

=== Princípio de funcionamento

O tensiômetro consiste em uma cápsula porosa, geralmente de cerâmica,
conectada a um medidor de vácuo por meio de um tubo, com todas as partes
preenchidas com água. A cápsula é permeável à água e aos solutos da solução do
solo, mas impermeável a gases e à matriz do solo, até determinado nível de
tensão @azevedo1999. Com o solo saturado, nenhuma água atravessa a cápsula e
não há vácuo. À medida que o solo seca, a água deixa o tubo através da
cápsula, criando em seu interior um vácuo equivalente à tensão da água no
solo, que é indicado pelo medidor; após uma chuva ou irrigação, o fluxo se
inverte, a água retorna do solo para o tubo e a leitura diminui @azevedo1999
@marouelli2008.

Em razão desse princípio, a leitura expressa a energia necessária para que a
água seja liberada das superfícies das partículas do solo, e não o seu
conteúdo de água. Uma mesma leitura, em solo argiloso ou arenoso, significa
que as plantas estão sujeitas à mesma energia de retenção; o que difere é o
tempo até o esgotamento da água armazenada, maior no solo argiloso, que retém
mais água para o mesmo nível de tensão @azevedo1999. Por isso o tensiômetro
dispensa calibração para cada tipo de solo @azevedo1999 --- o que não se
confunde com a calibração do instrumento de leitura, que relaciona a indicação
do medidor à pressão efetivamente presente no tubo e é tratada na
@sec-metrologia. Diz-se, por analogia, que o tensiômetro mede o "esforço" que
a planta exerce para extrair a água do solo @marouelli2008. A grandeza medida,
porém, é uma propriedade do estado da água no solo, que afeta a absorção pelas
plantas @marouelli2008, e não uma medida tomada na própria planta.

=== Limites de operação

O funcionamento do tensiômetro depende da formação de vácuo em seu interior, e
o limite superior de leitura é dado pelo ponto em que a água, sob vácuo, entra
em cavitação, isto é, passa a formar acentuadamente bolhas de vapor, com
pronunciada redução da sensibilidade das medidas. Esse limite diminui com a
altitude do local e com a temperatura da água, e toma-se em geral o valor de
80 kPa como leitura máxima de operação @azevedo1999 @marouelli2008. A fração
da água disponível que essa faixa alcança depende do solo: em latossolo de
cerrado, a faixa útil de 0 a 85 kPa compreende mais de 65% da água disponível
às plantas @azevedo1999, ao passo que, como referência geral, 30% a 60% da
água disponível do solo está retida acima de 80 kPa, fora do alcance do
instrumento @marouelli2008. A restrição pesa pouco no manejo, pois a
absorção de água pelas plantas frequentemente começa a cair antes que o
potencial na zona radicular ultrapasse o intervalo prático de medida do
tensiômetro @azevedo1999, e o instrumento é recomendado para o manejo da
irrigação da maioria das hortaliças @marouelli2008.

Duas propriedades da cápsula condicionam o desempenho do instrumento. A
pressão de borbulhamento é a pressão mínima de ar, aplicada à cápsula
saturada de água, na qual o ar começa a atravessar os seus poros; ela está
ligada ao diâmetro do maior poro e deve ser de pelo menos 100 kPa, pois até
esse valor a cápsula deve ser permeável apenas à água e aos íons
@azevedo1999. A definição pressupõe a cápsula saturada, e por isso o preparo
do instrumento inclui a escorva, isto é, a retirada do ar presente nos poros
da cápsula @marouelli2008; a consequência de operar com a cápsula
dessaturada foi observada no decorrer deste trabalho e é relatada no
@cap-prototipo. A condutância hidráulica, por sua vez, é a vazão de água
através da parede da cápsula por unidade de diferença de pressão, e determina
a velocidade de resposta do tensiômetro às variações de umidade do solo
@azevedo1999.

Por fim, a leitura no topo do instrumento difere da tensão no nível da
cápsula pelo peso da coluna de água contida no tubo. A correção
correspondente, de 0,098 kPa por centímetro de coluna @azevedo1999, e o seu
sinal na convenção adotada neste trabalho são apresentados na @sec-validacao.

=== Estanqueidade e manutenção <sec-estanqueidade>

Em pleno funcionamento, o tensiômetro opera à semelhança de uma câmara
hermeticamente fechada, e a verificação de sua integridade consiste,
basicamente, em detectar pontos de entrada de ar na faixa normal de operação,
de 0 a 80 kPa @azevedo1999. Entre os problemas mais comuns estão o vazamento
na tampa, nas conexões ou no medidor, que permite a entrada de ar no
aparelho, e a tampa rachada pela exposição ao sol. Uma leitura que permanece
em zero indica a necessidade de reabastecer o instrumento e submetê-lo a um
teste de sucção, para verificar se há penetração de ar pelas conexões ou pela
cápsula @azevedo1999.

O preparo do instrumento visa eliminar o ar de todo o sistema. Recomenda-se o
enchimento com água destilada, ou fervida e resfriada, para retirar os gases
dissolvidos; a saturação da cápsula por imersão durante dias; e a escorva com
bomba de vácuo manual, para a qual uma seringa do tipo hospitalar, de 20 mL ou
mais, pode ser adaptada @azevedo1999 @marouelli2008. A estanqueidade é
verificada por evaporação: com o tubo cheio e tampado e a cápsula exposta ao
ar, o vacuômetro deve registrar 50 kPa ou mais após cinco a oito horas, sem
acúmulo excessivo de ar no topo @azevedo1999; reimersa a cápsula, a leitura
deve cair rapidamente para a faixa de 0 a 5 kPa, e, se não cair, o
tensiômetro deve ser revisado quanto à entrada de ar e testado novamente
@marouelli2008.

Mesmo um instrumento estanque acumula ar com o uso. A água que retorna do solo
traz consigo ar dissolvido, que se acumula na parte superior do tubo ao longo
dos ciclos de secagem e umedecimento, dificulta a resposta do aparelho às
variações de umidade e leva a leituras abaixo da tensão real. Por isso a
manutenção periódica, que retira esse ar e completa o nível com água, é parte
da operação, com frequência recomendada entre três dias e uma semana
@azevedo1999 @marouelli2008. Esses requisitos são de natureza hidráulica, e
não eletrônica: um transdutor exato, acoplado a um instrumento que admite ar,
mede corretamente uma pressão que já não representa a tensão da água no solo.

=== Tensiometria eletrônica

Apesar da sua eficácia, o tensiômetro mecânico possui o gargalo de exigir
leituras visuais diárias, impossibilitando a telemetria nativa. A solução
técnica fundamentada pela IoT para contornar esse problema é a tensiometria
eletrônica. Ao acoplar um transdutor de pressão à câmara de ar do
instrumento, converte-se a pressão de vácuo em um sinal elétrico
@abdelmoneim2023.

A leitura eletrônica do tensiômetro tem antecedente na própria literatura
agronômica. #cite(<azevedo1999>, form: "prose") descrevem, com base em
Campbell e Mulla, um tensiômetro composto apenas pela cápsula e por um tubo
cheio de água, tampado na extremidade superior por uma rolha de borracha, ao
qual um transmissor eletrônico de pressão é conectado, durante a leitura, por
uma agulha inserida através da rolha; desse modo, um único medidor atende a um
grande número de tensiômetros. É o mesmo princípio do tensiômetro de punção,
lido por um vacuômetro portátil denominado tensímetro @marouelli2008. O
arranjo deste trabalho emprega essa derivação por septo perfurado, mas com a
agulha mantida no lugar e o transdutor dedicado a um único instrumento, o que
troca a economia do medidor compartilhado pela leitura contínua e remota
(@cap-prototipo).

== Transdução piezorresistiva da pressão <sec-transducao>

A piezorresistência é a variação da condutividade elétrica de um material
submetido a tensão mecânica, fenômeno a que Cookson aplicou esse nome em 1935
@barlian2009. Nos sensores de pressão, elementos piezorresistivos são
dispostos sobre um diafragma que se deforma com a pressão aplicada. Já nos
primeiros sensores comerciais, quatro desses elementos eram ligados em uma
ponte de Wheatstone com os quatro braços ativos, arranjo que fornece uma
tensão de saída proporcional à variação relativa de resistência,
$Delta R \/ R$, aumenta a sensibilidade, permite anular a saída e oferece uma
correção de primeira ordem para o deslocamento do zero com a temperatura
@barlian2009.

A temperatura afeta os piezorresistores por alterar a mobilidade e o número
de portadores de carga, o que modifica tanto a resistência quanto os
coeficientes piezorresistivos, isto é, a sensibilidade. Alguma dependência
térmica é inevitável em sensores de silício, mas pode ser compensada pela
própria ponte e por circuitos de condicionamento @barlian2009. O transdutor
adotado neste trabalho segue esse modelo: combina um sensor de pressão
piezorresistivo de silício e um circuito integrado de aplicação específica
que entrega um sinal analógico amplificado, compensado em temperatura entre
0 e 60 °C, com interface ratiométrica, isto é, com tensão de saída
proporcional à tensão de alimentação @cfsensor. O fabricante declara o
componente calibrado para a faixa especificada @cfsensor; a relação
disponível ao usuário, no entanto, é a curva nominal de catálogo, com o erro
máximo declarado, e não um certificado de calibração do exemplar adquirido.

Nos transdutores de saída analógica, como o adotado, o sinal é uma tensão
contínua que varia linearmente com a pressão, lida pelo conversor
analógico-digital (ADC) de um microcontrolador e convertida em quilopascais
(kPa) por uma função de calibração. A exatidão do resultado não é propriedade
do transdutor isolado: depende dessa função e do padrão de referência contra
o qual ela é estabelecida. O comportamento ratiométrico tem, além disso, uma
consequência direta. A tensão de alimentação é, no vocabulário da metrologia,
uma grandeza de influência: não afeta a pressão efetivamente medida, mas
afeta a relação entre a indicação e o resultado de medição @inmetro2012. Uma
variação da alimentação desloca a leitura sem que a pressão tenha mudado, de
modo que, na ausência de correção, uma calibração vale apenas para a
alimentação sob a qual foi realizada. A temperatura do transdutor é outra
grandeza de influência da mesma natureza.

== Fundamentos de metrologia <sec-metrologia>

A avaliação de um instrumento de medição exige vocabulário preciso, porque
termos de uso corrente, como exatidão, precisão e calibração, designam
conceitos distintos. Este trabalho adota as definições do Vocabulário
Internacional de Metrologia (VIM), na edição luso-brasileira publicada pelo
Inmetro @inmetro2012.

O erro de medição é a diferença entre o valor medido de uma grandeza e um
valor de referência. A exatidão de medição é o grau de concordância entre um
valor medido e um valor verdadeiro do mensurando; não é uma grandeza e não
lhe é atribuído valor numérico, e uma medição é dita mais exata quando
fornece um erro de medição menor. A precisão de medição, por sua vez, é o
grau de concordância entre indicações ou valores medidos obtidos por medições
repetidas, no mesmo objeto ou em objetos similares, sob condições
especificadas, e é geralmente expressa por medidas como o desvio-padrão, a
variância ou o coeficiente de variação @inmetro2012. Os dois conceitos são
independentes: leituras muito próximas entre si podem estar todas deslocadas
do valor verdadeiro. É pela mesma razão que a resolução nominal do conversor
analógico-digital de um microcontrolador não equivale à sua exatidão
(@sec-infraestrutura).

As condições sob as quais a precisão é avaliada distinguem três conceitos. A
repetibilidade é a precisão sob condições que incluem o mesmo procedimento,
os mesmos operadores, o mesmo sistema de medição, as mesmas condições de
operação e o mesmo local, com medições repetidas durante um curto período de
tempo. A precisão intermediária mantém o procedimento e o local, mas abrange
um período extenso de tempo e pode incluir outras condições submetidas a
mudanças, como novas calibrações, padrões, operadores e sistemas de medição. A
reprodutibilidade, enfim, refere-se a condições que incluem diferentes
locais, diferentes operadores e diferentes sistemas de medição @inmetro2012.

A incerteza de medição é o parâmetro não negativo que caracteriza a dispersão dos valores
atribuídos a um mensurando, com base nas informações utilizadas; ela inclui
componentes provenientes de efeitos sistemáticos, como os associados a
correções e aos valores atribuídos a padrões @inmetro2012. O Guia para a
Expressão da Incerteza de Medição (GUM) distingue dois métodos de avaliação: o
do tipo A, pela análise estatística de séries de observações, e o do tipo B,
por outros meios que não essa análise @jcgm2008. O desvio-padrão de leituras
repetidas do transdutor é, assim, informação do tipo A, ao passo que o erro
máximo declarado em catálogo e a classe de exatidão do vacuômetro de
referência, discutidos na @sec-validacao, entram na avaliação como
informações do tipo B.

A deriva instrumental é a variação da indicação ao longo do tempo, contínua
ou incremental, devida a variações nas propriedades metrológicas de um
instrumento de medição; ela não está relacionada a uma variação na grandeza
medida, nem a uma variação de qualquer grandeza de influência identificada
@inmetro2012. A definição tem uma consequência prática: atribuir à deriva uma
variação observada na leitura exige antes descartar as grandezas de
influência conhecidas, como a temperatura e a alimentação do transdutor.

Um padrão de medição é a realização da definição de uma grandeza, com valor
determinado e incerteza de medição associada, utilizada como referência. A
calibração é a operação que, sob condições especificadas, estabelece, numa
primeira etapa, uma relação entre os valores fornecidos por padrões, com suas
incertezas, e as indicações correspondentes e, numa segunda etapa, utiliza
essa informação para estabelecer uma relação que permita obter um resultado
de medição a partir de uma indicação; ela pode ser expressa, entre outras
formas, por uma função ou por uma curva de calibração. A calibração não se
confunde com o ajuste de um sistema de medição, conjunto de operações
efetuadas no sistema para que ele forneça indicações prescritas, como o
ajuste de zero e o de ganho; a calibração é, aliás, pré-requisito do ajuste
@inmetro2012. No protótipo, a calibração é registrada como os coeficientes de
uma função que o servidor aplica a cada leitura recebida; o transdutor em si
não é ajustado.

Quando a relação entre a indicação e o valor de referência é linear, a função
de calibração pode ser obtida pelo método dos mínimos quadrados, que
determina o intercepto e a inclinação da reta minimizando a soma dos
quadrados dos desvios entre os valores observados e os previstos pela reta. O
GUM ilustra o procedimento com a calibração de um termômetro e mostra como as
variâncias estimadas dos dois parâmetros, e a covariância entre eles, são
usadas para obter o valor e a incerteza-padrão de uma correção prevista pela
curva @jcgm2008. Note-se que o termo ajuste, no sentido estatístico de ajuste
de uma reta aos pares observados, não corresponde ao ajuste de um sistema de
medição definido acima; é no primeiro sentido que ele aparece no procedimento
de calibração descrito na @sec-validacao.

== Infraestrutura computacional de hardware e software <sec-infraestrutura>

A materialização de um nó sensor agrícola requer hardware robusto, de baixo
consumo energético e custo acessível. O microcontrolador ESP32 é empregado em
sistemas agrícolas de baixo custo por integrar, no mesmo módulo, processamento
e conectividade Wi-Fi e Bluetooth nativas @abdelmoneim2023 @correaquiroz2025
@purnama2024. O microcontrolador possui dois conversores analógico-digitais de
12 bits. A resolução nominal, contudo, não equivale a exatidão: o fabricante
descreve o conversor como sensível a ruído, a ponto de produzir grandes
discrepâncias entre leituras @espressifadc44, e a sua tensão de referência
varia entre exemplares @espressifcali, o que exige filtragem e correção no
firmware, conforme detalhado no @cap-prototipo.

Em relação à camada de software e comunicação, a conexão entre o hardware no
campo e o usuário é frequentemente mediada pelo MQTT (_Message Queuing
Telemetry Transport_), protocolo de publicação e assinatura concebido para a
comunicação entre máquinas, ou pelo próprio HTTP (_Hypertext Transfer
Protocol_), protocolo de requisição e resposta predominante na web
@naik2017. Os dados processados no microcontrolador são remetidos a
plataformas de nuvem ou servidores dedicados. No lado do cliente, painéis e
aplicativos móveis traduzem o estado do sistema em informação acionável,
permitindo ao produtor acompanhar as tendências da umidade do solo, os
alarmes e as ações recomendadas @ghazi2025 --- no caso deste trabalho, o
histórico e os limiares de tensão da água no solo, acompanhados remotamente
para apoiar a decisão de irrigar.

=== O estilo arquitetural REST

O HTTP é frequentemente empregado segundo o estilo arquitetural REST
(_Representational State Transfer_), definido por
#cite(<fielding2000>, form: "prose") como um conjunto de restrições impostas à
interação entre os componentes de um sistema distribuído. A primeira é a
arquitetura cliente-servidor, cujo princípio é a separação de
responsabilidades: separar a interface com o usuário do armazenamento dos
dados permite que os componentes evoluam de forma independente. A segunda
exige que a comunicação seja sem estado: cada requisição do cliente ao
servidor deve conter toda a informação necessária para ser compreendida, sem
recorrer a contexto armazenado no servidor, de modo que o estado da sessão é
mantido inteiramente no cliente. Somam-se a essas a possibilidade de
armazenar respostas em cache, a interface uniforme entre os componentes ---
traço central que distingue o estilo dos demais ---, o sistema em camadas e,
opcionalmente, o código sob demanda @fielding2000.

O termo é empregado neste trabalho em sentido menos estrito. A ingestão de
leituras é sem estado, pois cada requisição do nó sensor carrega a sua própria
credencial; a sessão do produtor no aplicativo, ao contrário, é mantida no
servidor (@sec-autorizacao), o que se afasta da segunda restrição.

=== Segurança do transporte

Segundo a especificação da versão 1.3, o TLS (_Transport Layer Security_) tem
como objetivo principal prover um canal seguro entre dois pares que se
comunicam, exigindo do transporte subjacente apenas um fluxo de dados
confiável e ordenado. O canal deve oferecer autenticação --- o servidor é
sempre autenticado, e o cliente, opcionalmente ---, confidencialidade, pois os
dados só são visíveis às extremidades, e integridade, pois não podem ser
modificados sem que a alteração seja detectada; essas propriedades devem
valer mesmo diante de um atacante com controle completo da rede. O protocolo
compõe-se de um protocolo de estabelecimento (_handshake_), que autentica as
partes, negocia os parâmetros criptográficos e estabelece o material de chave
compartilhado, e de um protocolo de registro, que usa esses parâmetros para
proteger o tráfego @rfc8446. Como a autenticação do cliente é opcional no
TLS, a identificação do nó sensor perante o servidor é feita, neste trabalho,
na camada de aplicação, por uma credencial própria (@sec-autorizacao).

=== Retransmissão e idempotência

Em um protocolo de requisição e resposta, uma falha de comunicação pode
ocorrer depois de o servidor processar a requisição e antes de o cliente ler a
resposta. O cliente não tem, então, como saber se a operação foi aplicada, e a
retransmissão pode duplicar o seu efeito. A especificação do HTTP trata esse
caso pela propriedade da idempotência: um método é idempotente se o efeito
pretendido, sobre o servidor, de várias requisições idênticas é o mesmo de uma
única requisição. Os métodos idempotentes distinguem-se justamente porque a
requisição pode ser repetida automaticamente quando uma falha de comunicação
ocorre antes que o cliente leia a resposta. O método POST não está entre os
métodos idempotentes da especificação, e um cliente não deve repetir
automaticamente uma requisição com método não idempotente, a menos que
disponha de um meio de saber que a semântica da requisição é, de fato,
idempotente, ou de detectar que a requisição original nunca foi aplicada
@rfc9110.

Essa ressalva é o ponto de partida do contrato de ingestão deste trabalho: a
requisição é tornada idempotente pela própria aplicação, por meio de uma
chave única por leitura, e a combinação da retransmissão até a confirmação com
o descarte de duplicatas no destino faz com que cada leitura seja gravada uma
única vez (@cap-prototipo).

=== Provisionamento por portal cativo

O termo portal cativo designa uma rede à qual um dispositivo pode se conectar
voluntariamente e cujo acesso permanece limitado até que certos requisitos
sejam cumpridos, tipicamente por meio do navegador. As implementações
anteriores à arquitetura proposta pelo IETF para esses portais alertam o
usuário modificando o tráfego DNS ou HTTP, prática que essa arquitetura
exclui, ao estabelecer que as soluções não devem exigir a falsificação de
respostas de servidores DNS ou HTTP @rfc8952. No provisionamento de
dispositivos embarcados, o mecanismo é reaproveitado com outro propósito: o
próprio dispositivo cria uma rede sem fio temporária e responde a toda
consulta DNS com o seu endereço, de modo que o celular que se conecta a ela é
conduzido à página de configuração. Trata-se da forma anterior descrita pelo
IETF, aqui empregada numa rede que existe apenas durante a configuração do nó
e que não serve de acesso à Internet (@sec-provisionamento).

=== Aplicação web progressiva

Dois mecanismos especificados pelo W3C sustentam o que se denomina aplicação
web progressiva (_Progressive Web App_, PWA). O manifesto de aplicação web é
um arquivo em formato JSON que reúne os metadados da aplicação --- nome,
ícones, endereço a ser aberto quando o usuário a inicia, modo de exibição e
escopo de endereços ---, com os quais o navegador pode oferecer uma
experiência de uso mais próxima à de um aplicativo nativo @w3cmanifest. O
_service worker_ é um contexto de execução separado da página, um _worker_,
que desperta para receber eventos, entre eles o de busca (_fetch_), correspondente às requisições de
rede da página; a especificação oferece ainda um repositório de requisições e
respostas, semelhante ao cache do HTTP, que facilita a construção de
aplicações capazes de funcionar sem conexão @w3cserviceworkers. Neste
trabalho, o _service worker_ é registrado sem cache de respostas: uma leitura
servida do cache seria exibida como se fosse atual, e distinguir a leitura
corrente de uma leitura antiga é requisito central da interface
(@sec-apresentacao).
