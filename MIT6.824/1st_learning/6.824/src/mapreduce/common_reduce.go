package mapreduce

import (
	"encoding/json"
	"log"
	"os"
	//"sort"
)

func doReduce(
	jobName string, // the name of the whole MapReduce job
	reduceTask int, // which reduce task this is
	outFile string, // write the output here
	nMap int, // the number of map tasks that were run ("M" in the paper)
	reduceF func(key string, values []string) string,
) {
	//
	// doReduce manages one reduce task: it should read the intermediate
	// files for the task, sort the intermediate key/value pairs by key,
	// call the user-defined reduce function (reduceF) for each key, and
	// write reduceF's output to disk.
	//
	// You'll need to read one intermediate file from each map task;
	// reduceName(jobName, m, reduceTask) yields the file
	// name from map task m.
	//
	// Your doMap() encoded the key/value pairs in the intermediate
	// files, so you will need to decode them. If you used JSON, you can
	// read and decode by creating a decoder and repeatedly calling
	// .Decode(&kv) on it until it returns an error.
	//
	// You may find the first example in the golang sort package
	// documentation useful.
	//
	// reduceF() is the application's reduce function. You should
	// call it once per distinct key, with a slice of all the values
	// for that key. reduceF() returns the reduced value for that key.
	//
	// You should write the reduce output as JSON encoded KeyValue
	// objects to the file named outFile. We require you to use JSON
	// because that is what the merger than combines the output
	// from all the reduce tasks expects. There is nothing special about
	// JSON -- it is just the marshalling format we chose to use. Your
	// output code will look something like this:
	//
	// enc := json.NewEncoder(file)
	// for key := ... {
	// 	enc.Encode(KeyValue{key, reduceF(...)})
	// }
	// file.Close()
	//
	// Your code here (Part I).
	//

	intermidiate_files := make([]os.File, nMap)
	json_decoders := make([]*json.Decoder, nMap)

	defer func() {
		for i := 0; i < nMap; i++ {
			intermidiate_files[i].Close()
		}
	}()

	for i := 0; i < nMap; i++ {
		filename := reduceName(jobName, i, reduceTask)
		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		json_decoders[i] = json.NewDecoder(file)
	}

	key_multivalues := make(map[string][]string, 1<<8)

	for i := 0; i < nMap; i++ {
		for json_decoders[i].More() {
			var kv KeyValue
			err := json_decoders[i].Decode(&kv)
			if err != nil {
				log.Fatal(err)
			}
			key_multivalues[kv.Key] = append(key_multivalues[kv.Key], kv.Value)
		}
	}

	key_result := make(map[string]string, len(key_multivalues))

	// invoke user `Reduce()`
	for key, value_list := range key_multivalues {
		key_result[key] = reduceF(key, value_list)
	}

	// output reduce result
	file, err := os.Create(outFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)

	for key, result := range key_result {
		err := enc.Encode(KeyValue{key, result})
		if err != nil {
			log.Fatal(err)
		}
	}

}
