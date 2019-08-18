# dist-traceroute
dist-traceroute is a distributed traceroute tool written in Go. It runs as a **Master server** and **distributed Slaves** (probes) on **remote machines**.

The **Slaves** are **config-less**, thus they get their instructions from the Master and report measurements back.
Example usage: `# sudo ./dist-traceroute-slave -master=localhost -name=slave1 -passwd=1234`

The **Master** holds the config in config-files, **instructs the Slaves** with their work and **collects measurements** from all Slaves.
Example usage: `# ./dist-traceroute-master`

# Master
The dist-traceroute Master acts as the single point of contact for all Slaves. 

The Master holds a list of allowed Slaves which may receive their configuration using their individual credentials.
Further to this, the Master holds the configuration of the **Targets**, that shall be **monitored by all Slaves**.
Data is transferred between Master and Slave via HTTP, the **Master** listens on **port 8990**.

## Usage on Master
```
# ./dist-traceroute-master -help
Usage:
  -config filename
    	Set config filename (default "./dt-slaves.json")
  -help
    	display this message
  -log /path/to/file
    	Logfile location /path/to/file (default "./dt-master.log")
  -loglevel warn, info, debug
    	Specify loglevel, one of warn, info, debug (default "info")
```

Example:
```
# ./dist-traceroute-master
```

## Example allowed Slaves config
The Master needs to know all slaves that shall be able to connect, they are stored in a configuration file (Default: dt-slaves.json). 
```
# cat dt-slaves.json
{
	"Slaves": 
	[
		{
			"Name": "slave1",
			"Password": "1234"
		},
		{
			"Name": "slave2",
			"Password": "123"
		}
	]
}
```
## Example Targets configuration
This config file (Default: dt-targets.json) holds all Targets, that shall be periodically probed by the Slaves.
```
{
        "ReportURL": "http://localhost:8990/results/post",
        "Targets": {
                "76d96640-d357-47b3-ba50-5248a7604bc9": {
                        "Name": "Google",
                        "Address": "www.google.at"
                },
                "fc06ab7c-ad6f-4837-b2f8-96ef95c4acee": {
                        "Name": "LKML",
                        "Address": "lkml.org"
                }
        },
        "Retries": 1,
        "MaxHops": 30,
        "TimeoutMs": 500
}
```

# Slave
The dist-traceroute slave are config-less probes and only need to be able to find their master server.

Slaves needs to be **run as root** to be able to conduct traceroute measurements.
It sends UDP datagrams and receives ICMP packets. For more details see https://github.com/aeden/traceroute

## Usage on Slave
```
# sudo ./dist-traceroute-slave -help
Usage:
  -help
    	display this message
  -log /path/to/file
    	Logfile location /path/to/file (default "./dt-slave.log")
  -loglevel warn, info, debug
    	Specify loglevel, one of warn, info, debug (default "info")
  -master hostname
    	Set the hostname/IP of the master server
  -master-port port (optional)
    	Set the listening port (optional) of the master server (default "8990")
  -name name
    	Unique name of this slave used on master for authentication and storage of results
  -passwd secret
    	Shared secret for slave on master
  -zDebugResults
    	Generate fake results, e.g. when run without root permissions
```

Example:
```
# sudo ./dist-traceroute-slave -master=localhost -name=slave1 -passwd=1234
```
