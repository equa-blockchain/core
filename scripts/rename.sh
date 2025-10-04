#!/bin/bash
# rename-to-equa.sh

echo "🔄 Renomeando projeto para EQUA..."

# Detectar sistema operacional
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    SED_INPLACE="sed -i ''"
else
    # Linux
    SED_INPLACE="sed -i"
fi

# Função para substituir em arquivo
replace_in_file() {
    local file=$1
    local search=$2
    local replace=$3

    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/$search/$replace/g" "$file"
    else
        sed -i "s/$search/$replace/g" "$file"
    fi
}

# Arquivos .go
echo "📝 Processando arquivos .go..."
for file in $(find . -type f -name "*.go"); do
    replace_in_file "$file" "ethereum" "equa"
    replace_in_file "$file" "Ethereum" "Equa"
    replace_in_file "$file" "ETHEREUM" "EQUA"
done

# Arquivos .md
echo "📝 Processando arquivos .md..."
for file in $(find . -type f -name "*.md"); do
    replace_in_file "$file" "ethereum" "equa"
    replace_in_file "$file" "Ethereum" "Equa"
    replace_in_file "$file" "ETHEREUM" "EQUA"
done

# go.mod
echo "📝 Processando go.mod..."
if [ -f "go.mod" ]; then
    replace_in_file "go.mod" "github.com/ethereum/go-ethereum" "github.com/SEU-USUARIO/equa-chain"
fi

# Renomear diretórios e arquivos
echo "📁 Renomeando diretórios..."
find . -depth -type d -name "*ethereum*" | while read dir; do
    newdir=$(echo "$dir" | sed 's/ethereum/equa/g')
    if [ "$dir" != "$newdir" ]; then
        mv "$dir" "$newdir"
        echo "  ✓ $dir → $newdir"
    fi
done

echo "✅ Renomeação completa!"
