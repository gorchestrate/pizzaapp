import logo from './logo.svg';
import React, { Component } from 'react';
import ReactHtmlParser from 'react-html-parser'; 
import Form from "@rjsf/material-ui";

import * as M from '@material-ui/core';


function getHandlers(obj) {
  var handlers = []
  if (obj != null && obj["Type"] == "event" && obj.hasOwnProperty("Handler") && obj.Handler.Type == "handler") {
    handlers.push(obj)
    return handlers
  }
  for (var property in obj) {
      if (obj.hasOwnProperty(property)) {
          if (typeof obj[property] == "object") {
            handlers= handlers.concat(getHandlers(obj[property]));
          } 
      }
  }
  return handlers
}


class JsonForm extends React.Component {
  constructor(props) {
      super(props);
      this.state = {
          workflow: {},
          definition: {
            Stmts: [],
            State:{}
          },
          svg: "",
      }
  }

  async loadWorkflow(){
    const urlSearchParams = new URLSearchParams(window.location.search);
    const params = Object.fromEntries(urlSearchParams.entries());
    var resp = await fetch(`/wf/pizza/`+params.id);
    var res = await resp.json();
    this.setState({
      workflow: res
    })
    console.log(res);

    var wfName = res.Meta.Workflow;

    var resp = await fetch(`/graph/`+wfName +"?format=svg");
    var res = await resp.text();
    this.setState({
      svg: res
    })
    console.log(res);

    var resp = await fetch(`/definition/`+wfName);
    var res = await resp.json();
    console.log(res);
    this.setState({
      definition: res
    })
  }

  componentDidMount() {
      this.loadWorkflow();
  }
//  /* { ReactHtmlParser (this.state.svg) }*/
  render() {
    console.log("RENDER")
    var handlers = getHandlers(this.state.definition.Stmts)
    var active = []
    if (this.state.definition == undefined || this.state.workflow.Meta == undefined ) {
      return (<div></div>)
    }
    console.log(this.state.workflow.Meta)
    if (this.state.workflow.Meta.Threads != null) {
      for(let thread of this.state.workflow.Meta.Threads) {
        if (thread.WaitEvents == null) {
          continue
        }
        for(let waitEvent of thread.WaitEvents) {
          for(let h of handlers) {
            if(waitEvent.Req.Name == h.Name) {
              active.push(h)
            }
          }
        }
      }
    }
    console.log(active)
    var forms = active.map((h) => {
      var uiSchema = {
       // "ui:title": h.
      }
      console.log(h.Handler)

      var onSubmit = async (data) =>  {
        console.log("SUBMITT", JSON.stringify(data.formData))
        var resp = await fetch('/wf/pizza/'+this.state.workflow.Meta.ID+"/"+h.Name, {
          method: 'POST', // or 'PUT'
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(data.formData),
        })
        var res = await resp.json();
        console.log("res", res)
        this.loadWorkflow();
      }
      var yourForm;
      var presubmit = (e) => {
        e.preventDefault();
        yourForm.submit()
        console.log("SUBMITTED", yourForm.submit)
      }
      return (
        <M.Grid item xs={12}><M.Paper style={{ padding: 10 }}> 
                    <Form schema={h.Handler.Input}
                    uiSchema={uiSchema}
                    onChange={log("changed")}
                    onSubmit={onSubmit}
                    ref={(form) => {yourForm = form;}}
                    onError={log("errors")}>
                   <div>
                      <M.Button color="primary" type="submit">
                        {h.Role} | {h.Name}
                      </M.Button>
                    </div>
                    </Form>
                  </M.Paper></M.Grid>
      )
    })

    // Render form data
    // Render Form for each available action (button for action with empty schema)
    // Submit form to API
    // Refresh workflow on 
      return (
              <M.Grid container spacing={5} justifyContent="space-around" direction="row">
            <M.Grid container xs={5} style={{ padding: 10 }} >
                  <Form schema={this.state.definition.State}
                  formData={this.state.workflow.State}
                  onChange={log("changed")}
                  onSubmit={log("submitted")}
                  onError={log("errors")}>
                    <M.Button color="primary" type="submit">Update Workflow State</M.Button>
                  </Form>
            </M.Grid>
            <M.Grid container item xs={5} spacing={2} direction="row" >
              {forms}
              </M.Grid>
            </M.Grid>
      );
  }
}

const schema = {
  title: "Todo",
  type: "object",
  required: ["title"],
  properties: {
    title: {type: "string", title: "Title", default: "A new task"},
    done: {type: "boolean", title: "Done?", default: false}
  }
};

const log = (type) => console.log.bind(console, type);

function App() {
  var onSubmit = async () => {
    console.log("SUBMIT")
    var id = (Math.random()*100000000000000000).toString()
    var resp = await fetch('/wf/pizza/'+id, {
      method: 'POST', // or 'PUT'
      headers: {
        'Content-Type': 'application/json',
      }
    })
    window.location.href = "/ui/?id=" + id;
  }
  return (

    <div>
    <M.AppBar position="static">
        <M.Toolbar>
            <M.Typography variant="title" color="inherit">
            Gorchestrate Workflow UI
            </M.Typography>
            <M.Button  style={{ "margin-left": 40 }} variant="contained" color="default" onClick={onSubmit}> New Workflow</M.Button>
        </M.Toolbar>
    </M.AppBar>
    <M.Container>
    <M.Paper elevation="2"><JsonForm /></M.Paper>
    </M.Container>
      
    </div>
  );
}

export default App;
