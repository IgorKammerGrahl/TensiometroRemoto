// Template ABNT NBR 14724 para TCC do IFC.
// Uso: #show: tcc.with(titulo: [...], autor: "...", ...)

// Seção pré-textual: título centralizado, sem numeração, fora do sumário.
#let secao-pretextual(titulo) = {
  pagebreak(weak: true)
  align(center, text(weight: "bold")[#upper(titulo)])
  v(1.5em)
}

#let tcc(
  titulo: [],
  autor: "",
  instituicao: "Instituto Federal Catarinense",
  curso: "",
  campus: "",
  cidade: "",
  ano: "",
  orientador: "",
  coorientador: none,
  natureza: [],
  resumo: none,
  palavras-chave: (),
  abstract: none,
  keywords: (),
  agradecimentos: none,
  body,
) = {
  set document(title: titulo, author: autor)
  set page(
    paper: "a4",
    margin: (top: 3cm, left: 3cm, bottom: 2cm, right: 2cm),
  )
  // Liberation Serif = métrica idêntica à Times New Roman
  set text(font: "Liberation Serif", size: 12pt, lang: "pt", region: "br")
  set par(leading: 1em, spacing: 1em, justify: true, first-line-indent: (amount: 1.25cm, all: true))
  set heading(numbering: "1.1")

  // Capítulos: página nova, caixa alta, negrito. Sem número (ex.: Referências) = centralizado.
  show heading.where(level: 1): it => {
    pagebreak(weak: true)
    set par(first-line-indent: 0cm)
    set align(if it.numbering == none { center } else { left })
    block(below: 1.5em)[#text(size: 12pt, weight: "bold")[
      #if it.numbering != none [#counter(heading).display(it.numbering)#h(0.75em)]#upper(it.body)
    ]]
  }
  show heading.where(level: 2): it => {
    set par(first-line-indent: 0cm)
    block(above: 1.5em, below: 1em)[#text(size: 12pt, weight: "bold")[
      #counter(heading).display(it.numbering)#h(0.75em)#upper(it.body)
    ]]
  }
  show heading.where(level: 3): it => {
    set par(first-line-indent: 0cm)
    block(above: 1.5em, below: 1em)[#text(size: 12pt, weight: "bold")[
      #counter(heading).display(it.numbering)#h(0.75em)#it.body
    ]]
  }

  // Identificação acima, fonte abaixo, para TODA ilustração. O template do IFC
  // é explícito: "independentemente do tipo de ilustração (quadro, desenho,
  // figura, fotografia, mapa, entre outros), a sua identificação aparece na
  // parte superior, precedida da palavra designativa" e, "após a ilustração,
  // na parte inferior, indicar a fonte consultada".
  show figure.where(kind: table): set figure.caption(position: top)
  show figure.where(kind: image): set figure.caption(position: top)
  set figure.caption(separator: [ – ])
  show figure.caption: set text(size: 10pt)
  show table: set par(justify: false, first-line-indent: 0cm)
  show table: set text(size: 10pt, hyphenate: false)

  set list(indent: 1.25cm)
  set enum(indent: 1.25cm)

  // ---------- CAPA ----------
  {
    set par(first-line-indent: 0cm, leading: 0.65em)
    align(center)[
      #image("Instituto_Federal_Catarinense_-_Marca_Vertical_2015.svg", height: 3cm)
      #v(0.5em)
      #text(weight: "bold")[#instituicao]\
      #curso\
      _Campus_ #campus

      #v(4cm)
      #text(weight: "bold")[#upper(autor)]
      #v(1fr)
      #par(leading: 1em)[#text(weight: "bold", hyphenate: false)[#upper(titulo)]]
      #v(1fr)
      #v(4cm)
      #cidade\
      #ano
    ]
  }

  // ---------- FOLHA DE ROSTO (página 1 da contagem) ----------
  pagebreak()
  counter(page).update(1)
  {
    set par(first-line-indent: 0cm, leading: 0.65em)
    align(center)[#text(weight: "bold")[#upper(autor)]]
    v(1fr)
    align(center)[#par(leading: 1em)[#text(weight: "bold", hyphenate: false)[#upper(titulo)]]]
    v(1fr)
    grid(
      columns: (1fr, 8cm),
      [],
      {
        set par(justify: true, leading: 0.65em)
        natureza
        par[Orientador: #orientador #if coorientador != none [\ Coorientador: #coorientador]]
      },
    )
    v(1fr)
    align(center)[
      #cidade\
      #ano
    ]
  }

  // ---------- FOLHA DE APROVAÇÃO ----------
  pagebreak()
  {
    set par(first-line-indent: 0cm, leading: 0.65em)
    align(center)[#text(weight: "bold")[#upper(autor)]]
    v(2em)
    align(center)[#par(leading: 1em)[#text(weight: "bold", hyphenate: false)[#upper(titulo)]]]
    v(3em)
    grid(
      columns: (1fr, 8cm),
      [],
      par(justify: true, leading: 0.65em)[
        Este Trabalho de Conclusão de Curso foi julgado adequado para a obtenção
        do título de Bacharel em Ciência da Computação e aprovado em sua forma
        final pelo curso de #curso do #instituicao -- _Campus_ #campus.
      ],
    )
    v(1fr)
    align(center)[
      #line(length: 8cm, stroke: 0.5pt)
      #orientador\
      Orientador -- IFC _Campus_ #campus
      #v(2em)
      #text(weight: "bold")[BANCA EXAMINADORA]
      #v(2em)
      #line(length: 8cm, stroke: 0.5pt)
      Prof.(ª) Nome completo, titulação\
      Instituição
      #v(2em)
      #line(length: 8cm, stroke: 0.5pt)
      Prof.(ª) Nome completo, titulação\
      Instituição
    ]
    v(1fr)
  }

  // ---------- AGRADECIMENTOS ----------
  if agradecimentos != none {
    secao-pretextual("Agradecimentos")
    agradecimentos
  }

  // ---------- RESUMO ----------
  if resumo != none {
    secao-pretextual("Resumo")
    resumo
    v(1em)
    par(first-line-indent: 0cm)[
      #text(weight: "bold", style: "italic")[Palavras-chave:]
      #emph(palavras-chave.join("; "))
    ]
  }

  // ---------- ABSTRACT ----------
  if abstract != none {
    secao-pretextual("Abstract")
    text(lang: "en")[#abstract]
    v(1em)
    par(first-line-indent: 0cm)[
      #text(weight: "bold", style: "italic", lang: "en")[Keywords:]
      #text(lang: "en")[#emph(keywords.join("; "))]
    ]
  }

  // ---------- SUMÁRIO ----------
  {
    pagebreak(weak: true)
    show outline.entry.where(level: 1): it => {
      v(0.5em, weak: true)
      strong(upper(it))
    }
    outline(title: align(center)[#text(size: 12pt)[SUMÁRIO]#v(1em)], indent: 1em)
  }

  // ---------- TEXTO ----------
  // Paginação visível a partir daqui (canto superior direito, contada desde a folha de rosto)
  set page(header: align(right, text(size: 10pt)[#context counter(page).display()]))
  body
}

// Fonte de tabela/figura (abaixo, ABNT)
#let fonte(texto) = align(center, text(size: 10pt)[Fonte: #texto])

// Marcadores de pendência — remover antes da entrega final.
#let pendente(texto) = text(fill: rgb("#b00020"), weight: "bold")[[#texto]]
