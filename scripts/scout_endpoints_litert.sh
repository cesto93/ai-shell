#!/bin/bash

# Configurazione
HOST="http://localhost"
PORT="9379"
BASE_URL="${HOST}:${PORT}"
MODEL_NAME="gemma-4-E2B-it.litertlm"

# Payload standard in stile OpenAI (grazie alla flag --api openai)
PAYLOAD="{\"model\": \"${MODEL_NAME}\", \"messages\": [{\"role\": \"user\", \"content\": \"ping\"}], \"prompt\": \"ping\"}"

echo "=========================================================="
echo "🕵️‍♂️  Inizio scansione endpoint su: ${BASE_URL}"
echo "📝 Nome Modello target: ${MODEL_NAME}"
echo "=========================================================="
echo ""

# 1. Lista di endpoint da testare in GET (Metadata, OpenAPI doc, Health checks)
get_endpoints=(
    "/"
    "/health"
    "/ping"
    "/openapi.json"
    "/swagger.json"
    "/v1/models"
    "/v1/models/${MODEL_NAME}"
)

# 2. Lista di endpoint da testare in POST (Inference)
post_endpoints=(
    "/v1/chat/completions"
    "/chat/completions"
    "/v1/completions"
    "/completions"
    "/v1/models/${MODEL_NAME}/chat/completions"
    "/v1/models/${MODEL_NAME}/completions"
    "/v1/engines/${MODEL_NAME}/completions"
    "/v1/engines/${MODEL_NAME}/chat/completions"
    "/v1beta/models/${MODEL_NAME}:generateContent"
    "/v1beta/models/${MODEL_NAME}:predict"
    "/v1beta/models/default:generateContent"
    "/${MODEL_NAME}/v1/chat/completions"
    "/api/v1/generate"
    "/api/generate"
    "/generate"
    "/chat"
)

echo "--- [ TEST ENDPOINT GET ] ---"
for ep in "${get_endpoints[@]}"; do
    url="${BASE_URL}${ep}"
    # Cattura solo l'HTTP status code
    status=$(curl -s -o /dev/null -w "%{http_code}" "$url")
    
    if [ "$status" != "404" ] && [ "$status" != "000" ]; then
        echo "[🟢 GET $status] -> $url"
    else
        echo "[❌ 404] -> $url"
    fi
done

echo ""
echo "--- [ TEST ENDPOINT POST ] ---"
for ep in "${post_endpoints[@]}"; do
    url="${BASE_URL}${ep}"
    status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$url" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD")
    
    if [ "$status" != "404" ] && [ "$status" != "000" ]; then
        echo -e "[🟢 POST \033[0;32m$status\033[0m] -> \033[1;34m$url\033[0m"
        
        # Mostra un'anteprima della risposta se non è un errore bloccante
        if [ "$status" == "200" ]; then
            echo "    ↳ Risposta server: $(curl -s -X POST "$url" -H "Content-Type: application/json" -d "$PAYLOAD" | head -n 3)..."
        fi
    else
        echo "[❌ 404] -> $url"
    fi
done

echo ""
echo "=========================================================="
echo "Scansione terminata. Controlla i log del tuo server per vedere quale chiamata ha risposto!"
