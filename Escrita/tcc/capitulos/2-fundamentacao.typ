= Fundamentação teórica

Este capítulo apresenta a base conceitual e tecnológica necessária para a
compreensão e o desenvolvimento deste trabalho de pesquisa. Para que a
integração de um protótipo de telemetria agrícola seja viável e eficiente, é
imprescindível compreender as tecnologias subjacentes e o contexto agronômico
no qual o sistema será inserido. Inicialmente, discute-se a evolução da
Agricultura de Precisão e da Internet das Coisas (IoT), que fornecem a
infraestrutura de comunicação do projeto. Em seguida, abordam-se os conceitos
de gestão hídrica inteligente e a relevância crítica da tensiometria
eletrônica como método fisiológico de medição do estresse hídrico. Por fim,
delineiam-se os componentes de hardware e software que fundamentam a
arquitetura de monitoramento proposta.

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
responsável por aproximadamente 70% a 80% do uso total desse recurso
@lakhiar2024. Diante da escassez hídrica impulsionada pelas mudanças
climáticas, os métodos de manejo convencionais provam-se insustentáveis. O
excesso de água não apenas desperdiça o recurso, como também lixivia
nutrientes vitais do solo e favorece o surgimento de doenças fitopatogênicas
@ghazi2025.

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

== Tensiometria eletrônica como método de monitoramento

Para que um sistema de suporte à irrigação atue com precisão, a métrica de
controle deve refletir adequadamente a necessidade hídrica da planta. Grande
parte das implementações de baixo custo utiliza sensores de umidade
volumétrica resistivos ou capacitivos genéricos, que mensuram apenas a
porcentagem de água presente em um dado volume de terra. Essa abordagem
apresenta limitações, pois a presença de água no solo não garante que ela
esteja prontamente disponível para as raízes, uma vez que solos mais
argilosos, por exemplo, retêm a água com muita força @abdelmoneim2023.

O tensiômetro, por sua vez, atua a partir do princípio do potencial
matricial. Em vez de medir o volume, ele mensura a força, ou seja, o esforço
de sucção (vácuo) que a planta precisa exercer para extrair a água retida
pelas partículas do solo. O instrumento clássico é composto por um tubo
selado preenchido com água e equipado com uma cápsula de cerâmica porosa na
extremidade. Conforme o solo seca ao redor da cápsula, a água é sugada do
interior do tubo, gerando uma tensão negativa (vácuo) proporcional ao esforço
da planta. Essa métrica fornece um parâmetro fisiológico direto do estresse
hídrico.

Apesar da sua eficácia, o tensiômetro mecânico possui o gargalo de exigir
leituras visuais diárias, impossibilitando a telemetria nativa. A solução
técnica fundamentada pela IoT para contornar esse problema é a tensiometria
eletrônica. Ao acoplar um transdutor de pressão piezorresistivo, converte-se
a pressão de vácuo mecânica em um sinal elétrico analógico @abdelmoneim2023.
O transdutor emite pulsos de tensão (Volts) que podem ser lidos diretamente
pelos conversores analógico-digitais (ADCs) de um microcontrolador,
traduzindo o sinal em medidas exatas de pressão (quilopascais -- kPa)
enviadas diretamente para a nuvem.

== Infraestrutura computacional de hardware e software

A materialização de um nó sensor agrícola requer hardware robusto, de baixo
consumo energético e custo acessível. O microcontrolador ESP32 tem se
destacado amplamente em pesquisas de redes de sensores agrícolas devido à sua
arquitetura _dual-core_ e, principalmente, por possuir módulos nativos de
conectividade Wi-Fi e Bluetooth @correaquiroz2025. O ESP32 possui múltiplos
conversores analógico-digitais de alta resolução (12 bits), garantindo uma
amostragem elétrica precisa das flutuações enviadas pelo transdutor do
tensiômetro, operando de forma estável para aquisição dos dados do campo
@purnama2024.

Em relação à camada de software e comunicação, a conexão entre o hardware no
campo e o usuário é frequentemente mediada por protocolos de transporte leves
projetados especificamente para IoT, como o MQTT (_Message Queuing Telemetry
Transport_) ou o HTTP (_Hypertext Transfer Protocol_). Os dados processados
no microcontrolador são remetidos a plataformas de nuvem ou servidores
dedicados. No lado do cliente, a construção de um aplicativo móvel atua como
uma interface humano-computador. Essa interface deve possuir requisitos
rígidos de usabilidade, fornecendo ao produtor a liberdade de monitorar os
históricos e os limiares de tensão do solo de forma gráfica e intuitiva,
caracterizando o acompanhamento remoto e ubíquo do estresse hídrico da
lavoura para a melhor tomada de decisão @ghazi2025.
