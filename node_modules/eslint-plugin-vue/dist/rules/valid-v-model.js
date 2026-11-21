const require_index = require("../utils/index.js");
// lib/rules/valid-v-model.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
const VALID_MODIFIERS = /* @__PURE__ */ new Set([
	"lazy",
	"number",
	"trim"
]);
/**
* Check whether the given node is valid or not.
*/
function isValidElement(node) {
	const name = node.name;
	return name === "input" || name === "select" || name === "textarea" || name !== "keep-alive" && name !== "slot" && name !== "transition" && name !== "transition-group" && require_index.default.isCustomComponent(node);
}
/**
* Check whether the given node is a MemberExpression containing an optional chaining.
* e.g.
* - `a?.b`
* - `a?.b.c`
*/
function isOptionalChainingMemberExpression(node) {
	return node.type === "ChainExpression" && node.expression.type === "MemberExpression";
}
/**
* Check whether the given node can be LHS (left-hand side).
*/
function isLhs(node) {
	switch (node.type) {
		case "TSAsExpression":
		case "TSNonNullExpression": return isLhs(node.expression);
		case "Identifier":
		case "MemberExpression": return true;
		default: return false;
	}
}
/**
* Check whether the given node is a MemberExpression of a possibly null object.
* e.g.
* - `(a?.b).c`
* - `(null).foo`
*/
function isMaybeNullObjectMemberExpression(node) {
	if (node.type !== "MemberExpression") return false;
	switch (node.object.type) {
		case "ChainExpression": return true;
		case "Literal": return node.object.value === null && !node.object.bigint;
		case "MemberExpression": return isMaybeNullObjectMemberExpression(node.object);
		default: return false;
	}
}
/**
* Get the variable by names.
*/
function getVariable(name, leafNode) {
	let node = leafNode;
	while (node != null) {
		const variables = node.variables;
		const variable = variables && variables.find((v) => v.id.name === name);
		if (variable != null) return variable;
		if (node.parent.type === "VDocumentFragment") break;
		node = node.parent;
	}
	return null;
}
var valid_v_model_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid `v-model` directives",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/valid-v-model.html"
		},
		fixable: null,
		schema: [],
		messages: {
			unexpectedInvalidElement: "'v-model' directives aren't supported on <{{name}}> elements.",
			unexpectedInputFile: "'v-model' directives don't support 'file' input type.",
			unexpectedArgument: "'v-model' directives require no argument.",
			unexpectedModifier: "'v-model' directives don't support the modifier '{{name}}'.",
			missingValue: "'v-model' directives require that attribute value.",
			unexpectedOptionalChaining: "Optional chaining cannot appear in 'v-model' directives.",
			unexpectedNonLhsExpression: "'v-model' directives require the attribute value which is valid as LHS.",
			unexpectedNullObject: "'v-model' directive has potential null object property access.",
			unexpectedUpdateIterationVariable: "'v-model' directives cannot update the iteration variable '{{varName}}' itself."
		}
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='model']"(node) {
			const element = node.parent.parent;
			const name = element.name;
			if (!isValidElement(element)) context.report({
				node,
				messageId: "unexpectedInvalidElement",
				data: { name }
			});
			if (name === "input" && require_index.default.hasAttribute(element, "type", "file")) context.report({
				node,
				messageId: "unexpectedInputFile"
			});
			if (!require_index.default.isCustomComponent(element)) {
				if (node.key.argument) context.report({
					node: node.key.argument,
					messageId: "unexpectedArgument"
				});
				for (const modifier of node.key.modifiers) if (!VALID_MODIFIERS.has(modifier.name)) context.report({
					node: modifier,
					messageId: "unexpectedModifier",
					data: { name: modifier.name }
				});
			}
			if (!node.value || require_index.default.isEmptyValueDirective(node, context)) {
				context.report({
					node,
					messageId: "missingValue"
				});
				return;
			}
			const expression = node.value.expression;
			if (!expression) return;
			if (isOptionalChainingMemberExpression(expression)) context.report({
				node: expression,
				messageId: "unexpectedOptionalChaining"
			});
			else if (!isLhs(expression)) context.report({
				node: expression,
				messageId: "unexpectedNonLhsExpression"
			});
			else if (isMaybeNullObjectMemberExpression(expression)) context.report({
				node: expression,
				messageId: "unexpectedNullObject"
			});
			for (const reference of node.value.references) {
				const id = reference.id;
				if (id.parent.type !== "VExpressionContainer") continue;
				if (getVariable(id.name, element) != null) context.report({
					node: expression,
					messageId: "unexpectedUpdateIterationVariable",
					data: { varName: id.name }
				});
			}
		} });
	}
};
exports.default = valid_v_model_default;
