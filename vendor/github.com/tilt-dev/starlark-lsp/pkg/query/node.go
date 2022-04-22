package query

const (
	NodeTypeModule              = "module"
	NodeTypeCall                = "call"
	NodeTypeArgList             = "argument_list"
	NodeTypeFunctionDef         = "function_definition"
	NodeTypeParameters          = "parameters"
	NodeTypeKeywordArgument     = "keyword_argument"
	NodeTypeIdentifier          = "identifier"
	NodeTypeIfStatement         = "if_statement"
	NodeTypeExpressionStatement = "expression_statement"
	NodeTypeForStatement        = "for_statement"
	NodeTypeAssignment          = "assignment"
	NodeTypeAttribute           = "attribute"
	NodeTypeString              = "string"
	NodeTypeDictionary          = "dictionary"
	NodeTypeList                = "list"
	NodeTypeComment             = "comment"
	NodeTypeBlock               = "block"
	NodeTypeERROR               = "ERROR"

	FieldName       = "name"
	FieldParameters = "parameters"
	FieldReturnType = "return_type"
	FieldBody       = "body"
)
