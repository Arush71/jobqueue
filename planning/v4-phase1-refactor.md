<!-- markdownlint-disable -->

### Places that assume Image-Processing handler.

- [x] CreateJob Handler.
- [x] response.go, Req Struct and Validate function.
- [x] job_mapper.go, both the functions.
- [x] Create Job sql query and Job Scheme: Job_type,Image Path and potentially params(tho that's just raw json)
- [x] GetJobByID handler, has a rigid return result type that assumes image.
- [x] Worker.go, On functions GetJobByID, manageJobImageProcessing.

## Thoughts on how you're gonna go about refactoring it to be dynamic.

1) Answer the 4 questions first, to figure out the plan.

2) Think about job.go, it heavily relies on job being image-processed and its not SUPER required as the db job type kinda does its job.

 ### Answer these questions, When done with figuring out where the code assumes image processing.

1) How do I want a job to say “I am type X” and “here are my params” in a way that works for any type, not just images?

2) Where should the knowledge of “how to validate and run a resize job” live? (In the API? In the worker? In a separate handler file?)

3) How should the database store params when they can be completely different shapes for different job types?

4) When I create a job via POST /jobs, what should the JSON body look like for a non-image job?

## Answers 

1) I probably want it to say that, via a seperate handler, that way it can be more customized and validated, cause if its truly dynamic and we use one handler for every job - it will be super losse and hard to be specific and validate. But the, do I create a handler for every job type OR have one main http handler but, find a way to distinguish the job type, again hard thing to do. 

2) In the job handler, not the http one but like a medium that's responsible for a certain job, for instance, thumbnail medium should validate and complete it on its own, as for where it should live, not sure probably the jobs package.

3) YES, this is a good question. my plan is to store it via a json object in db, which is loose, but, to compensate for that, we validate the job riggrously on both the http end and the processing end via its job "handler".

4) that's where it gets interesting, do I have one http handler like /jobs and then have diff types of jobs depending on the body(seems fine to me), or do I have a new handler for every job type? maybe the former is better. As for the json body, it should have a field saying what type of job, its options and payload.(assuming we go with the one handler route.)


### What am I gonna do?

- [x] I plan to use one http handledr for all job types. There would be a jobHandler interface that will have a feew methods that each job handler has to satisfy. One of those methods would be a validate function, using which I will validat the payload. 

- [x] there would be a register map that has the type string to a jobHandler type, which I would use to identify which job handler does this job request belong to and/or if it's even valid.

- [x] the other jobHandler interface functions would be a Process function which tries to process the job, and returns a potential error or data, this would also take in a context, so that worker can exert more control.

- [x] The database would store the jobType and the payload, the latter of which would be a JSONB. To make it more riggrious, I'm gonna use the job handler's validate function on both the http and the worker layer.


## In which order am I gonna do things?

- [x] I'll fix the job schema, remove the fields i don't need and add those that I do. Also fix the create_job query to be more dynamic.

- [x] I'll work on creating the handlerJob interface type, and the first job handler(img procerssing), and create the register to hook them up.

- [x] I'll work on the CreateJob handler and make it use the new inteface and validate the payload via that.

- [x] I'll then work on worker.go, refactor it so it uses the Process function, and change the GetJobByID stuff and all the places where it assumes a image-based system.

- [x] I'll refactor any other place that assumes job-image processing, for instance, GetJobByID handler and etc.
