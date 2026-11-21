const require_index = require("../utils/index.js");
// lib/rules/require-typed-object-prop.ts
const checkPropIdentifierType = (context, identifierNode) => {
	if (identifierNode.name !== "Object" && identifierNode.name !== "Array") return;
	const arrayTypeSuggestion = identifierNode.name === "Array" ? "[]" : "";
	context.report({
		node: identifierNode,
		messageId: "expectedTypeAnnotation",
		suggest: [{
			messageId: "addTypeAnnotation",
			data: { type: `any${arrayTypeSuggestion}` },
			fix(fixer) {
				return fixer.insertTextAfter(identifierNode, ` as PropType<any${arrayTypeSuggestion}>`);
			}
		}, {
			messageId: "addTypeAnnotation",
			data: { type: `unknown${arrayTypeSuggestion}` },
			fix(fixer) {
				return fixer.insertTextAfter(identifierNode, ` as PropType<unknown${arrayTypeSuggestion}>`);
			}
		}]
	});
};
const checkPropArrayType = (context, arrayNode) => {
	for (const elementNode of arrayNode.elements) if (elementNode?.type === "Identifier") checkPropIdentifierType(context, elementNode);
};
const checkPropObjectType = (context, objectNode) => {
	const typeProperty = objectNode.properties.find((prop) => prop.type === "Property" && prop.key.type === "Identifier" && prop.key.name === "type");
	if (!typeProperty || typeProperty.type !== "Property") return;
	if (typeProperty.value.type === "Identifier") checkPropIdentifierType(context, typeProperty.value);
	else if (typeProperty.value.type === "ArrayExpression") checkPropArrayType(context, typeProperty.value);
};
const checkProp = (prop, context) => {
	if (prop.type !== "object") return;
	switch (prop.node.value.type) {
		case "Identifier":
			checkPropIdentifierType(context, prop.node.value);
			break;
		case "ArrayExpression":
			checkPropArrayType(context, prop.node.value);
			break;
		case "ObjectExpression":
			checkPropObjectType(context, prop.node.value);
			return;
	}
};
var require_typed_object_prop_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce adding type declarations to object props",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/require-typed-object-prop.html"
		},
		fixable: null,
		hasSuggestions: true,
		schema: [],
		messages: {
			expectedTypeAnnotation: "Expected type annotation on object prop.",
			addTypeAnnotation: "Add `{{ type }}` type annotation."
		}
	},
	create(context) {
		const filename = context.filename;
		if (!require_index.default.isVueFile(filename) && !require_index.default.isTypeScriptFile(filename)) return {};
		if (require_index.default.isVueFile(filename)) {
			const sourceCode = context.sourceCode;
			const documentFragment = sourceCode.parserServices.getDocumentFragment && sourceCode.parserServices.getDocumentFragment();
			if (!documentFragment) return {};
			if (documentFragment.children.filter((element) => require_index.default.isVElement(element) && element.name === "script").every((script) => !require_index.default.hasAttribute(script, "lang", "ts"))) return {};
		}
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefinePropsEnter(_node, props) {
			for (const prop of props) checkProp(prop, context);
		} }), require_index.default.executeOnVue(context, (obj) => {
			const props = require_index.default.getComponentPropsFromOptions(obj);
			for (const prop of props) checkProp(prop, context);
		}));
	}
};
exports.default = require_typed_object_prop_default;
