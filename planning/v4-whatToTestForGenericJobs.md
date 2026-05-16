<!-- markdownlint-disable -->

### What tests to write, aka what do you wanna test as of rn.

I've addead generic jobs, to be frank there's not much testing, that's different from a single job based job-queue but, some that I can think of are as follows:

- [x] Valid job. This path should test correct behaviour with no error.(check for all 2 job handlers) 
- [x] Invalid data. This path should fail with MAX RETRIES, test it for all jobs, pass invalid data.
- [x] Invalid job, send a invalid job, should fail at the handler verification process. retry count 0. 
- [x] Job timeout. A valid job, but with a timeout of over 30s so it should be cancelled.
- [x] job restart recovery.
