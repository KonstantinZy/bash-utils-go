#!/bin/bash
FILENAME="result.txt"
printf "Execute script several times\nWithout options\n"
printf "++++++++++++++++++++++++++++++ Start script ++++++++++++++++++++++++++++++ \n" > $FILENAME
for i in 0 1 2 3 4 5
do
    if (($i == 0)); then
        opt=""
    else
        printf -v opt "%sm %u " "-" $i
    fi

    if (($i != 0)); then
        printf "option %s\n" "$opt"
    fi
    comand="go run multiwc.go $opt sample1.testdata sample.testdata sample.testdata sample1.testdata"
    printf "++++++++++++++++++++++++++++++ option $opt ++++++++++++++++++++++++++++++ \n" >> $FILENAME
    eval "$comand >> $FILENAME"
    printf "%s Execution time %s\n" "------------------------" "------------------------" >> $FILENAME
    for j in 1 2 3
    do
        printf "Calculating execution time %d time\n" "$j"
        eval "$comand | grep \"Execution time duration:\" >> $FILENAME"
    done
done

: <<'END'
printf "Multiline comment"
END