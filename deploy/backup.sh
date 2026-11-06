#!/bin/sh
# Dump diario do banco com retencao. Roda no servico `backup` do compose, com
# PGHOST/PGUSER/PGDATABASE/PGPASSWORD no ambiente.
#
# Formato custom (-Fc): comprimido, e restaurado com pg_restore -- ver
# docs/DEPLOY.md, "Restaurar um backup".
set -eu

DIR=/backups
RETENCAO_DIAS=${RETENCAO_DIAS:-14}

# Sobra de um dump interrompido (container parado no meio).
rm -f "$DIR"/*.tmp

while :; do
	arq="$DIR/telemetria-$(date -u +%Y-%m-%dT%H%M%SZ).dump"

	# Grava em .tmp e renomeia: dump interrompido nunca parece backup.
	if pg_dump -Fc -f "$arq.tmp"; then
		mv "$arq.tmp" "$arq"
		echo "backup: $arq ($(du -h "$arq" | cut -f1))"

		# Retencao SO depois de um dump bom. Com o banco fora do ar por dias,
		# os antigos ficam -- seriam os unicos que restaram.
		# -mtime +13 = 14 dias ou mais.
		find "$DIR" -name 'telemetria-*.dump' -mtime +"$((RETENCAO_DIAS - 1))" -print -delete
	else
		rm -f "$arq.tmp"
		echo "backup: pg_dump FALHOU; nenhum backup antigo foi apagado" >&2
	fi

	sleep 86400
done
