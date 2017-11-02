# watch-shell

Watch the output of commands on remote hosts in a convenient, tabular format.

```
USAGE

	watch-shell <command> host1 [host2 ...]

EXAMPLES

	watch-shell date host1 host2 host3
	watch-shell 'hostname ; du -hs $HOME' host1 host2 host3
	cat hosts.txt | xargs watch-shell 'hostname ; ps -p $(pidof prometheus) -o %cpu,%mem'
```

