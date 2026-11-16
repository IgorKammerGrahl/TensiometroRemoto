package simulador

import (
	"context"
	"errors"
	"fmt"
)

// Provisionar garante que o estado tenha pelo menos n nos, cadastrando pela
// API os que faltam. Os nos novos sao distribuidos pelos talhoes em rodizio.
//
// salvar e chamado depois de CADA cadastro, antes da calibracao: o token so
// existe na resposta do POST, e um corte entre o cadastro e a gravacao
// deixaria no servidor um no cujo token ninguem mais tem. Gravar mil vezes o
// arquivo custa segundos; perder o token custa o no.
func Provisionar(ctx context.Context, cli *Cliente, e *Estado, salvar func() error,
	n int, prefixo string, talhoes []string) (criados int, err error) {

	if faltam := n - len(e.Nos); faltam > 0 && len(talhoes) == 0 {
		return 0, fmt.Errorf("o estado tem %d no(s) e foram pedidos %d: informe -talhao "+
			"para cadastrar os %d que faltam", len(e.Nos), n, faltam)
	}
	for i := len(e.Nos); i < n; i++ {
		id := fmt.Sprintf("%s-%04d", prefixo, i+1)
		talhao := talhoes[i%len(talhoes)]
		token, err := cli.CriarNo(ctx, id, fmt.Sprintf("Nó simulado %04d", i+1), talhao)
		if err != nil {
			return criados, err
		}
		// ProximoSeq 1: o seq do firmware tambem comeca em 1.
		e.Nos = append(e.Nos, No{ID: id, Token: token, TalhaoID: talhao, ProximoSeq: 1})
		if err := salvar(); err != nil {
			return criados, fmt.Errorf("no %s cadastrado, mas o estado nao foi gravado (o token "+
				"dele esta perdido): %w", id, err)
		}
		criados++
	}
	// Calibracao num segundo passo, que tambem completa o no que ficou sem
	// ela numa execucao interrompida entre o cadastro e a calibracao.
	for i := range e.Nos[:n] {
		if e.Nos[i].CalibracaoID != "" {
			continue
		}
		cal, err := cli.CriarCalibracao(ctx, e.Nos[i].ID)
		if err != nil {
			return criados, err
		}
		e.Nos[i].CalibracaoID = cal
		if err := salvar(); err != nil {
			return criados, err
		}
	}
	return criados, nil
}

// ErrNoNaoSimulado e o motivo de abortar antes do primeiro envio.
var ErrNoNaoSimulado = errors.New("no sem a marca de simulado")

// Verificar confere, pela API e imediatamente antes da carga, que todo no
// que vai enviar esta cadastrado como simulado, visivel ao usuario e ativo.
//
// E A BARREIRA QUE IMPEDE LEITURA GERADA NA SERIE DE UM NO REAL. O estado e
// um arquivo editavel: um token de no fisico colado nele por engano mandaria
// sinal sintetico para a serie do tensio-01, com cara de medida, e nada no
// banco acusaria. Por isso a marca e conferida no servidor, e nao no arquivo,
// e qualquer divergencia aborta a execucao inteira -- nenhum no envia se um
// deles estiver errado.
//
// A conferencia e por id. O token nao e verificavel pelo plano de usuario;
// um token de outro no, com o id de um no simulado, sai daqui aprovado e
// morre no servidor com 403 "device_id nao corresponde ao token", sem
// gravar nada.
func Verificar(ctx context.Context, cli *Cliente, nos []No) error {
	vistos, err := cli.Nos(ctx)
	if err != nil {
		return err
	}
	for _, no := range nos {
		d, ok := vistos[no.ID]
		switch {
		case !ok:
			return fmt.Errorf("o no %s do estado nao aparece para este usuario (outra instancia, "+
				"outro usuario ou talhao sem concessao). Nada foi enviado", no.ID)
		case !d.Simulado:
			return fmt.Errorf("%w: %s esta cadastrado no servidor como no REAL. O simulador nao "+
				"escreve em serie de no real. Nada foi enviado", ErrNoNaoSimulado, no.ID)
		case !d.Ativo:
			return fmt.Errorf("o no %s esta desativado: todo envio dele voltaria 403. "+
				"Reative-o pela tela ou use outro estado. Nada foi enviado", no.ID)
		case no.CalibracaoID == "":
			return fmt.Errorf("o no %s nao tem calibracao no estado. Nada foi enviado", no.ID)
		}
	}
	return nil
}
