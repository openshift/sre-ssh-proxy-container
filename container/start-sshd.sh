#!/bin/bash

function handle_signal {
  PID=$!
  echo "received signal. PID is ${PID}"
  kill -s SIGHUP $PID
}

trap "handle_signal" SIGINT SIGTERM SIGHUP


# add sre-user to /etc/passwd
USER_ID="$(id -u)"
GROUP_ID="$(id -g)"
# sleepysh is non-interactive but never exits
echo "sre-user::${USER_ID}:${GROUP_ID}:SRE USER:/home/sre-user:/bin/sleepysh" >> /etc/passwd

# setup sre-user home dir
mkdir /home/sre-user
mkdir /home/sre-user/.ssh
chmod 700 /home/sre-user
chmod 700 /home/sre-user/.ssh

# setup SSHD
mkdir /opt/sshd
cp /opt/ssh_files/sshd_config /opt/sshd
chmod 700 /opt/sshd
chmod 600 /opt/sshd/sshd_config
echo "generating sshd keys..."
/opt/sshd-keygen


echo "starting sshd"
/usr/sbin/sshd -f /opt/sshd/sshd_config -D -e
echo "stopping sshd"
