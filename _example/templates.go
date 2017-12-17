package main

import (
	"fmt"

	"github.com/flosch/pongo2"
	"go.uber.org/config"
)

func main() {
	playbook := `
---

- hosts: all
  gather_facts: False
  remote_user: vault
  vars:
  - remotedir : "/data/m2/repository/"

  tasks:
  - name: delete mvn cache stat
    stat: path="{{remotedir}}/{{item}}"
    with_items: '{{delete|default([])}}'
    register: delete_configfile

  - name: delete mvn cache delete
    file: dest="{{item.stat.path}}" state=absent
    with_items: '{{delete_configfile.results}}'
    when: (item.skipped is not defined or not item.skipped) and item.stat.exists
`

	p, _ := config.NewYAMLProviderFromBytes([]byte(playbook))
	_ = p
	// fmt.Println(p.Get("").Value())

	pongo2.RegisterTag("banned_tag", tagSandboxDemoTagParser)
	pongo2.DefaultSet.BanTag("banned_tag")

	pongo2.NewSet("aaaa", loader pongo2.TemplateLoader)
	// Compile the template first (i. e. creating the AST)
	tpl, err := pongo2.DefaultSet.FromString("Hello {{ name|capfirst }}  {{remotedir}}/{{item}}!")
	if err != nil {
		panic(err)
	}

	// Now you can render the template with the given
	// pongo2.Context how often you want to.
	out, err := tpl.Execute(pongo2.Context{"name": "fred", "remotedir": "'11111111111111'"})
	if err != nil {
		panic(err)
	}
	fmt.Println(out) // Output: Hello Fred!

}

type tagSandboxDemoTag struct {
}

func (node *tagSandboxDemoTag) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	writer.WriteString("hello")
	return nil
}

func tagSandboxDemoTagParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	fmt.Println(doc)
	return &tagSandboxDemoTag{}, nil
}
