#!/usr/bin/env bash

# 當遇到超時或 API 錯誤中斷時，自動重試的最大次數
MAX_RETRIES=10
RETRY_COUNT=0

echo "🚀 開始執行 Goal 任務（帶自動重試護航）..."

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  # 非互動模式執行，或者使用 --permission-mode auto
  claude -c -p "請檢查當前進度並繼續執行 /goal 中的剩餘任務，直到完全完成"
  
  EXIT_CODE=$?

  if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ 任務已成功完成！"
    exit 0
  else
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "⚠️ 檢測到 API 超時或網絡錯誤中斷 (Exit code: $EXIT_CODE)。"
    echo "🔄 正在進行第 $RETRY_COUNT/$MAX_RETRIES 次自動重試，5 秒後繼續..."
    sleep 5
  fi
done

echo "❌ 已達最大重試次數，請檢查網絡或 API 狀態。"
