const require_index = require("../utils/index.js");
// lib/rules/no-duplicate-attr-inheritance.ts
/**
* @fileoverview Disable inheritAttrs when using v-bind="$attrs"
* @author Hiroki Osame
*/
function isConditionalGroup(elements) {
	const firstElement = elements[0];
	const lastElement = elements.at(-1);
	if (!lastElement || elements.length < 2) return false;
	const inBetweenElements = elements.slice(1, -1);
	return require_index.default.hasDirective(firstElement, "if") && (require_index.default.hasDirective(lastElement, "else-if") || require_index.default.hasDirective(lastElement, "else")) && inBetweenElements.every((element) => require_index.default.hasDirective(element, "else-if"));
}
function isMultiRootNodes(elements) {
	return elements.length > 1 && !isConditionalGroup(elements);
}
var no_duplicate_attr_inheritance_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce `inheritAttrs` to be set to `false` when using `v-bind=\"$attrs\"`",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-duplicate-attr-inheritance.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: { checkMultiRootNodes: { type: "boolean" } },
			additionalProperties: false
		}],
		messages: { noDuplicateAttrInheritance: "Set \"inheritAttrs\" to false." }
	},
	create(context) {
		const shouldCheckMultiRootNodes = (context.options[0] || {}).checkMultiRootNodes === true;
		let shouldInheritAttrs = true;
		const attrsRefs = [];
		function processOptions(node) {
			const inheritAttrsProp = require_index.default.findProperty(node, "inheritAttrs");
			if (inheritAttrsProp && inheritAttrsProp.value.type === "Literal") shouldInheritAttrs = Boolean(inheritAttrsProp.value.value);
		}
		return require_index.default.compositingVisitors(require_index.default.executeOnVue(context, processOptions), require_index.default.defineScriptSetupVisitor(context, { onDefineOptionsEnter(node) {
			if (node.arguments.length === 0) return;
			const define = node.arguments[0];
			if (define.type !== "ObjectExpression") return;
			processOptions(define);
		} }), require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='bind'][key.argument=null] > VExpressionContainer"(node) {
			if (!shouldInheritAttrs) return;
			const reference = node.references.find((reference) => reference.variable == null && reference.id.name === "$attrs");
			if (reference) attrsRefs.push(reference);
		} }), { "Program:exit"(program) {
			const element = program.templateBody;
			if (element == null) return;
			const rootElements = element.children.filter(require_index.default.isVElement);
			if (!shouldCheckMultiRootNodes && isMultiRootNodes(rootElements)) return;
			for (const attrsRef of attrsRefs) context.report({
				node: attrsRef.id,
				messageId: "noDuplicateAttrInheritance"
			});
		} });
	}
};
exports.default = no_duplicate_attr_inheritance_default;
