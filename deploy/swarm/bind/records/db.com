$TTL    86400
@       IN      SOA     ns1.com. admin.com. (
                        2026041801
                        3600
                        1800
                        604800
                        86400 )
@       IN      NS      ns1.com.
ns1     IN      A       10.161.34.51
