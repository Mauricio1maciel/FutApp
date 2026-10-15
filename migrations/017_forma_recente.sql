-- Forma recente na classificação: os últimos 5 resultados de cada time naquela
-- competição ("VVEDV", do mais antigo para o mais recente), calculada junto com a tabela.
ALTER TABLE standings ADD COLUMN IF NOT EXISTS form VARCHAR(5) NOT NULL DEFAULT '';
