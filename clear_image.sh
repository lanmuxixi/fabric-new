docker images | grep "none"   | awk '{print $3}' | xargs docker rmi --force
docker images | grep "dev-vp" | awk '{print $3}' | xargs docker rmi --force