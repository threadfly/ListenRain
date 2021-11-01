# benchmark description
The main purpose of the benchmark test is to test listenrain's packet
forwarding ability, so when writing test cases, the impact of non-framework
factors is reduced as much as possible, so several points are mainly
considered:
- Memory reuse reduces GC impact
- Simplified logic of codec module
- Use local environment testing to reduce the impact of real network
- Currently only supports benchmark testing of a single link
