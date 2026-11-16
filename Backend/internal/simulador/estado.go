package simulador

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Estado e o que o simulador precisa lembrar entre execucoes.
//
// O token de um no e entregue uma unica vez, no cadastro, e o servidor so
// guarda o HMAC. Sem este arquivo, cada execucao teria de cadastrar nos novos
// e a lista do produtor cresceria a cada rodada. Pelo mesmo motivo o arquivo
// e 0600 e fica fora do repositorio (.gitignore): sao credenciais, ainda que
// de nos que so existem em software.
type Estado struct {
	// URL base da API em que os nos foram cadastrados. Token de uma
	// instancia nao autentica em outra; misturar as duas so produziria 401
	// em massa com cara de falha do servidor.
	URL string `json:"url"`
	Nos []No   `json:"nos"`
}

type No struct {
	ID           string `json:"id"`
	Token        string `json:"token"`
	CalibracaoID string `json:"calibracao_id"`
	TalhaoID     string `json:"talhao_id"`
	// ProximoSeq e o primeiro seq ainda nao reservado. Ver Reservar.
	ProximoSeq int64 `json:"proximo_seq"`
}

// CarregarEstado devolve um estado vazio quando o arquivo nao existe.
func CarregarEstado(caminho string) (*Estado, error) {
	bruto, err := os.ReadFile(caminho)
	if errors.Is(err, fs.ErrNotExist) {
		return &Estado{}, nil
	}
	if err != nil {
		return nil, err
	}
	var e Estado
	if err := json.Unmarshal(bruto, &e); err != nil {
		return nil, fmt.Errorf("%s nao e um estado do simulador: %w", caminho, err)
	}
	return &e, nil
}

// Salvar grava num temporario e renomeia: um corte no meio da escrita deixa
// o arquivo anterior inteiro, e nao um JSON truncado que perderia todos os
// tokens de uma vez.
func (e *Estado) Salvar(caminho string) error {
	bruto, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(caminho), ".simulador-estado-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no caminho feliz o rename ja o levou
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(bruto); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), caminho)
}

// Reservar separa para cada no um bloco de n seq e devolve o inicio de cada
// bloco. O estado que sai daqui ja tem ProximoSeq depois do bloco, e e esse
// estado que o chamador salva ANTES do primeiro envio.
//
// E a regra do firmware ("avanca e persiste ANTES de enviar") aplicada a uma
// execucao inteira. Se o processo morrer no meio, a proxima execucao comeca
// depois do bloco: o que sobrou vira buraco na sequencia, que e diagnostico,
// e nunca um seq reaproveitado -- que seria uma leitura nova descartada como
// duplicata, em silencio. Devolver o que nao foi usado e trabalho de
// Devolver, no fim de uma execucao que terminou inteira.
func (e *Estado) Reservar(nos int, n int64) []int64 {
	inicios := make([]int64, nos)
	for i := range nos {
		inicios[i] = e.Nos[i].ProximoSeq
		e.Nos[i].ProximoSeq += n
	}
	return inicios
}

// Devolver recua ProximoSeq para logo depois do ultimo seq usado por cada no.
// So e seguro depois que todos os envios terminaram.
func (e *Estado) Devolver(inicios []int64, usados []int64) {
	for i := range inicios {
		e.Nos[i].ProximoSeq = inicios[i] + usados[i]
	}
}
