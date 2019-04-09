package main

import (
	"context"
	"fmt"
	"time"

	"github.com/project-flogo/rules/common"
	"github.com/project-flogo/rules/common/model"
	"github.com/project-flogo/rules/ruleapi"

	//gw dependencies
	"github.com/project-flogo/contrib/activity/channel"
	"github.com/project-flogo/contrib/activity/log"
	channeltrigger "github.com/project-flogo/contrib/trigger/channel"
	flogoapi "github.com/project-flogo/core/api"
	"github.com/project-flogo/core/engine"
	"github.com/project-flogo/core/engine/channels"
	"github.com/project-flogo/microgateway"
	microapi "github.com/project-flogo/microgateway/api"
)

func main() {

	fmt.Println("** Example usage of embedded microgateway action call from rulesapp **")

	//Load the tuple descriptor file (relative to GOPATH)
	tupleDescAbsFileNm := common.GetAbsPathForResource("src/github.com/project-flogo/rules/examples/rulesapp/rulesapp.json")
	tupleDescriptor := common.FileToString(tupleDescAbsFileNm)

	fmt.Printf("Loaded tuple descriptor: \n%s\n", tupleDescriptor)
	//First register the tuple descriptors
	err := model.RegisterTupleDescriptors(tupleDescriptor)
	if err != nil {
		fmt.Printf("Error [%s]\n", err)
		return
	}

	//Create a RuleSession
	rs, _ := ruleapi.GetOrCreateRuleSession("asession")

	//// check for name "Bob" in n1
	rule := ruleapi.NewRule("n1.name == Bob")
	rule.AddCondition("c1", []string{"n1"}, checkForBob, nil)
	rule.SetAction(checkForBobAction)
	rule.SetContext("This is a test of context")
	rs.AddRule(rule)
	fmt.Printf("Rule added: [%s]\n", rule.GetName())

	//Start the rule session
	rs.Start(nil)

	//Now assert a "n1" tuple
	fmt.Println("Asserting n1 tuple with name=Tom")
	t1, _ := model.NewTupleWithKeyValues("n1", "Tom")
	t1.SetString(nil, "name", "Tom")
	rs.Assert(nil, t1)

	//Now assert a "n1" tuple
	fmt.Println("Asserting n1 tuple with name=Bob")
	t2, _ := model.NewTupleWithKeyValues("n1", "Bob")
	t2.SetString(nil, "name", "Bob")
	rs.Assert(nil, t2)

	//Retract tuples
	rs.Retract(nil, t1)
	rs.Retract(nil, t2)

	//delete the rule
	rs.DeleteRule(rule.GetName())

	//unregister the session, i.e; cleanup
	rs.Unregister()
}

func checkForBob(ruleName string, condName string, tuples map[model.TupleType]model.Tuple, ctx model.RuleContext) bool {
	//This conditions filters on name="Bob"
	t1 := tuples["n1"]
	if t1 == nil {
		fmt.Println("Should not get a nil tuple in FilterCondition! This is an error")
		return false
	}
	name, _ := t1.GetString("name")
	return name == "Bob"
}

func checkForBobAction(ctx context.Context, rs model.RuleSession, ruleName string, tuples map[model.TupleType]model.Tuple, ruleCtx model.RuleContext) {
	fmt.Printf("Rule fired: [%s]\n", ruleName)
	fmt.Printf("Context is [%s]\n", ruleCtx)

	//gw channel for channel trigger
	var err error
	_, err = channels.New("to_gw", 5)
	if err != nil {
		panic(err)
	}

	//gw channel for channel activity
	_, err = channels.New("from_gw", 5)
	if err != nil {
		panic(err)
	}

	gateway := microapi.New("Embedded")

	//Add Log step
	service := gateway.NewService("log", &log.Activity{})
	service.SetDescription("Invoking Log Service")
	step := gateway.NewStep(service)
	step.AddInput("message", "Output: test log message service invoked")

	//Add channel activity step
	service = gateway.NewService("channel", &channel.Activity{})
	service.SetDescription("Invoking Channel Service")
	step = gateway.NewStep(service)
	step.AddInput("channel", "from_gw")
	step.AddInput("data", "microgateway invoked successfully!")

	//Setup callback
	ch_from_gw := channels.Get("from_gw")

	var data_from_gw interface{}

	ch_from_gw.RegisterCallback(func(data interface{}) {
		data_from_gw = data
	})

	flogoapp := flogoapi.NewApp()

	channeltrg := flogoapp.NewTrigger(&channeltrigger.Trigger{}, nil)
	channelhandler, err := channeltrg.NewHandler(&channeltrigger.HandlerSettings{
		Channel: "to_gw",
	})
	if err != nil {
		panic(err)
	}

	settings, err := gateway.AddResource(flogoapp)
	if err != nil {
		panic(err)
	}

	_, err = channelhandler.NewAction(&microgateway.Action{}, settings)
	if err != nil {
		panic(err)
	}

	e, err := flogoapi.NewEngine(flogoapp)
	if err != nil {
		panic(err)
	}

	//start the flogo app
	go engine.RunEngine(e)

	//wait for flogo engine to start up gw
	time.Sleep(2 * time.Second)

	//send an event to channeltrigger to invoke gw steps
	ch_to_gw := channels.Get("to_gw")
	ch_to_gw.Publish("test")

	//wait for data to be sent back via call back
	time.Sleep(2 * time.Second)

	//val, found := data_from_gw
	if data_from_gw == nil {
		fmt.Println("Error executing microgateway")
		return
	}
	fmt.Printf("%v\n", data_from_gw)
}
