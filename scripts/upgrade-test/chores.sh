source ./util.sh

ACTION=$1
case $ACTION in
  "cleanup")
    nukeServices
    ;;

  "start")
    startServices
    ;;

  "restart")
    restartServices
    ;;

  "stop")
    stopServices
    ;;

  *)
    echo "Missing required argument: cleanup, start, restart, stop"
    ;;
esac
