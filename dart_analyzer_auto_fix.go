package main

import (
    "bufio"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "sort"
)

type analyzer_info struct {
    rule string
    file string
    line int
}

func readLine(path string) <-chan string {
    out := make(chan string)    

    go func() {
        inFile, _ := os.Open(path)
        defer inFile.Close()
        scanner := bufio.NewScanner(inFile)
        scanner.Split(bufio.ScanLines) 

        for scanner.Scan() {
            out <- scanner.Text()
        }
        close(out)
    }()
    return out
}

func parse(input <-chan string) <-chan analyzer_info {
    out := make(chan analyzer_info)
    
    go func() {        
        for st := range input {
            pattern := regexp.MustCompile(`.*\((?P<name>\w+)\sat\s\[\w+\]\s(?P<file>.*)\:(?P<line>\d+)\)$`)
            match := pattern.FindStringSubmatch(st)
            if (match != nil)  {
                ln, _ := strconv.Atoi(match[3])
                out <- analyzer_info{rule:match[1],file:match[2],line:ln}
            }            
        }
        close(out)
    }()
    return out
}


func stats_by_files_and_rules(input <-chan analyzer_info) (map[string][]analyzer_info, map[string][]analyzer_info) {
    files := make(map[string][]analyzer_info)
    rules := make(map[string][]analyzer_info)

    for inf := range input {
        files[inf.file] = append(files[inf.file], inf)
        rules[inf.rule] = append(rules[inf.rule], inf)
    }            
    return files, rules
}

func read_all_lines(file string) ([]string) {
    var lines []string
    for str := range readLine(file) {
        lines = append(lines, str)
    }
    return lines
}

func write_all_lines(file string, lines *[]string){
      f, err := os.Create(file)
      if err != nil {
        fmt.Println(err)
      }
      defer f.Close()

      w := bufio.NewWriter(f)
      for _, line := range *lines {
        fmt.Fprintln(w, line)
      }
      w.Flush()    
}


func rule_omit_local_variable_types(lines *[]string, line int) (int) {
    line_before :=  (*lines)[line]
    line_after :=  line_before

    patternVariable := regexp.MustCompile(`(^\s*)(\S+)(\s+\w+\s*=.*$)`)
    line_after = patternVariable.ReplaceAllString(line_before, "${1}var${3}")

    if (line_before == line_after) {
        patternConst := regexp.MustCompile(`(^\s*)(const|final)(\s*)(\S+)(\s+)(\w+\s*=.*$)`)
        line_after = patternConst.ReplaceAllString(line_before, "${1}${2}${3}${6}")
    }

    if (line_before == line_after) {
        patternFor := regexp.MustCompile(`(^\s*for\s*\(\s*)(\S+)(\s+\w+\s*(=|in).*)$`)
        line_after = patternFor.ReplaceAllString(line_before, "${1}var${3}")
    }

/*    fmt.Println(line,"before: ",line_before)
    fmt.Println(line,"after : ",line_after)*/
    
    (*lines)[line] = line_after

    if line_after != line_before {
        return 1
    }    
    fmt.Println(line,"NOT PROCESSED: ",line_before)    

    return 0
}

func rule_prefer_final_locals(lines *[]string, line int) (int) {
    line_before :=  (*lines)[line]
    line_after :=  line_before

    if strings.Index(line_before, " var ") > 0 {
        line_after = strings.Replace(line_before, " var ", " final ", 1)
    } else {
        pattern := regexp.MustCompile(`^\s*(?P<name>\w+)`)
        match := pattern.FindStringSubmatch(line_before)
        if (match != nil)  {
            line_after = strings.Replace(line_before, match[1], "final "+match[1], 1)
        }                
    }

/*    fmt.Println(line,"before: ",line_before)
    fmt.Println(line,"after : ",line_after)*/
    
    (*lines)[line] = line_after

    if line_after != line_before {
        return 1
    }    
    fmt.Println(line,"NOT PROCESSED: ",line_before)

    return 0
}


func process_file(file string, infos []analyzer_info) int{
    lines := read_all_lines(file)
    nmodified := 0

    if len(lines)==0 {
         return 0
    } 

    fmt.Println(file, len(infos), len(lines))
    
    for inf := range infos {
        rule := infos[inf].rule
        line := infos[inf].line - 1 
        switch rule {
        case "prefer_final_locals":
            nmodified = nmodified + rule_prefer_final_locals(&lines, line)
        case "omit_local_variable_types":
            nmodified = nmodified + rule_omit_local_variable_types(&lines, line)
        }
    }

    if nmodified > 0 {
        write_all_lines(file, &lines)
    }
    return nmodified;
}


type Pair struct {
  Key string
  Value int
}

type PairList []Pair

func main() {  
    project_root := filepath.Dir(os.Args[1])
    analyzer_res_file_name := os.Args[2]
    rule_name := os.Args[3]
    fmt.Println(project_root)
    fmt.Println(analyzer_res_file_name)
    fmt.Println(rule_name)

    files , rules := stats_by_files_and_rules(parse(readLine(analyzer_res_file_name)))

    fmt.Println("By rules:")    
    rulesCount := make(PairList, len(rules))
    i := 0
    for rule, infs := range rules {
        rulesCount[i] = Pair{rule, len(infs)}
        i++        
    }
    sort.Slice(rulesCount, func(i, j int) bool { return rulesCount[i].Value > rulesCount[j].Value })
    for kv := range rulesCount {
        fmt.Println(rulesCount[kv].Key, rulesCount[kv].Value)
    }

    fixedCount := 0
    for file, infs := range files {
        sort.Slice(infs, func(i, j int) bool { return infs[i].line > infs[j].line })

        rules_inf := make([]analyzer_info, 0)

        for inf := range infs {
            if infs[inf].rule == rule_name {
                rules_inf = append(rules_inf, infs[inf])
            }
        }            

        if len(rules_inf) > 0 {
            fixedCount = fixedCount + process_file(filepath.Join(project_root, file), rules_inf)
        }
    }

    fmt.Println("FIXED:", fixedCount)        
}
