FROM centos:centos7

# creature comforts
ENV LANG en_US.UTF-8
ENV CONTAINER docker 
ENV USER sre-user
ENV HOME /home/sre-user
ENV TERM xterm
#WORKDIR /root
ADD bashrc /root/.bashrc


RUN yum install -y vim-enhanced initscripts openssh-server openssh-clients

ADD pam_sshd /etc/pam.d/sshd
RUN chmod 644 /etc/pam.d/sshd && chown root.root /etc/pam.d/sshd

ADD list-authorized-keys /etc/ssh/list-authorized-keys
RUN chmod 755 /etc/ssh/list-authorized-keys

ADD sleepysh /bin/sleepysh
RUN chmod 755 /bin/sleepysh

RUN useradd sre-user && \
    sed -i '/sre-user/d' /etc/passwd
RUN rm -rf /home/sre-user

# needed for openshift
RUN chgrp -R 0 /opt && \
    chmod g+w /etc/passwd && \
    chmod  g+rwX /home && \
    chmod  g+rwX /opt

#RUN chgrp -R 1000 /opt && \
#    chmod 775 /opt

RUN mkdir /opt/ssh_files && chmod 775 /opt/ssh_files
ADD sshd_config /opt/ssh_files
ADD bashrc /opt/ssh_files
ADD bash_profile /opt/ssh_files
RUN chmod 664 /opt/ssh_files/sshd_config

ADD start-sshd.sh /opt
ADD sshd-keygen /opt

USER 1000
WORKDIR  /opt
CMD ["/opt/start-sshd.sh"]
