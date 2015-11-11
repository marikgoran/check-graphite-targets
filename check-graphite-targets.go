package main

import (
    "fmt"
    "net/http"
    "log"
    "io/ioutil"
    "strings"
    "strconv"
    "flag"
    "os"
)

func debug( what string, something interface{} ){
    fmt.Printf ("Debug: %s: %#v\n",what,something)
}

func main() {

    agg_flag := flag.String("a","min","Aggregation method for multiple values per target: min,max,avg")
    meta_flag := flag.String("A","any","Aggregation method for multiple targets: any,all")
    host_flag := flag.String("H","localhost","Graphite hostname")
    http_flag := flag.Bool("s",false,"Use https (self-sign certs not implemented yet)")
    target_flag := flag.String("t","None","Target (metric) to query")
    from_flag := flag.String("f","5min","Time interval to query")
    w_flag := flag.Float64("w",0,"Warning threshold")
    c_flag := flag.Float64("c",0,"Error threshold")
    flag.Parse()

   if len(os.Args) < 9 {
        flag.PrintDefaults()
        os.Exit(1)
    }
    var protocol string
    if *http_flag {
        protocol="https"
    } else {
        protocol="http"
    } 
    url := fmt.Sprintf("%s://%s/render/?target=%s&from=-%s&format=raw", protocol, *host_flag,*target_flag,*from_flag)
    //debug ("URL",url)

    resp,err := http.Get(url)
    if err != nil {
        log.Fatal(err)
    }

    defer resp.Body.Close()

    data,err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }

    outputAll := strings.Split(strings.TrimSpace(string(data)),"\n")

    var final_msg string
    var final_perfdata string
    var final_code int

    for i,output := range outputAll {
        status_msg,status_code,perfdata := check_target (output,*agg_flag,*c_flag,*w_flag)
        if i==0 { 
            final_code=status_code
            final_msg=status_msg
        } 

        // In case of the "any" aggregate, the highest exit code will be the final code
        // for the "all", it is the lowest code that is final
        if *meta_flag == "all" {
            if status_code < final_code { 
                final_code = status_code
                final_msg = status_msg
            }
        } else {
            if status_code > final_code {
                final_code = status_code
                final_msg = status_msg
            }
        }
        final_perfdata = final_perfdata + perfdata
    }
    fmt.Printf ("%s status | %s\n", final_msg, strings.Replace(final_perfdata,"\n"," ",-1))
    os.Exit(final_code)

}

func check_target(target,agg string, c,w float64 ) (status_msg string,status_code int,perfdata string) {
    
    var measured_value float64
    var num_values int

    values:=strings.Split(strings.Split(target,"|")[1],",")
    for _,value := range values {
        num_value,err := strconv.ParseFloat(value,64) 
        if err != nil {
        // Null is not a value. Use transformNull in the target for different behaviour
            continue
        }
        
        num_values++
        if num_values==1 && agg!="avg" {  // initialize the measured value in order get min/max aggregation
            measured_value=num_value
        }

        switch agg {
        case "min":
            if num_value < measured_value {
                measured_value = num_value
            }
        case "max":
            if num_value > measured_value {
                measured_value = num_value
            }
        case "avg":
            measured_value = measured_value + num_value
        }        
    }

    if agg == "avg" {
        measured_value=measured_value/float64(num_values)
    }
       
    perfdata = fmt.Sprintf("%s=%f\n",strings.Split(target,",")[0],measured_value)
    
    // try to infer what is the critical range based on the warning threshold
    if c < w {
        if measured_value <= c {
            return "CRITICAL",2,perfdata
        }
        if measured_value < w {
            return "WARNING",1,perfdata
        }
        return "OK",0,perfdata
    } else {
        if measured_value >= c {
            return "CRITICAL",2,perfdata
        }
        if measured_value > w {
            return "WARNING",1,perfdata
        }
        return "OK",0,perfdata
    }
}
