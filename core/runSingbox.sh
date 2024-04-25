#!/bin/bash

set -e

tokill=$$

runSingbox(){
  ./sing-box run &
  tokill=$!
}

terminateSingbox()
{
  if kill -0 $tokill > /dev/null 2>&1; then
    echo "Terminating singbox PID=$tokill"
    kill $tokill
    while kill -0 $tokill > /dev/null 2>&1; do
      sleep 1
    done
  fi
}

trap terminateSingbox SIGINT SIGTERM SIGKILL

runSingbox

while true
do
    sleep 5
    if [ -f "signal" ]; then
        signal=`cat signal`
        echo "Signal received: $signal"
        # Remove singnal file
        rm -f signal >> /dev/null 2>&1
        case ${signal} in 
            "stop")
                terminateSingbox
                ;;
            "restart")
                terminateSingbox
                runSingbox
                ;;
        esac
    fi
done