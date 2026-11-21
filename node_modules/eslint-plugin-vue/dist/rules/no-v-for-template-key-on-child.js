const require_index = require("../utils/index.js");
// lib/rules/no-v-for-template-key-on-child.ts
/**
* @author Yosuke Ota
* This rule is based on X_V_FOR_TEMPLATE_KEY_PLACEMENT error of Vue 3.
* see https://github.com/vuejs/vue-next/blob/b0d01e9db9ffe5781cce5a2d62c8552db3d615b0/packages/compiler-core/src/errors.ts#L70
*/
/**
* Check whether the given attribute is using the variables which are defined by `v-for` directives.
*/
function isUsingIterationVar(vFor, vBindKey) {
	if (vBindKey.value == null) return false;
	const references = vBindKey.value.references;
	const variables = vFor.parent.parent.variables;
	return references.some((reference) => variables.some((variable) => variable.id.name === reference.id.name && variable.kind === "v-for"));
}
var no_v_for_template_key_on_child_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow key of `<template v-for>` placed on child elements",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-v-for-template-key-on-child.html"
		},
		fixable: null,
		schema: [],
		messages: { vForTemplateKeyPlacement: "`<template v-for>` key should be placed on the `<template>` tag." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement[name='template'] > VStartTag > VAttribute[directive=true][key.name.name='for']"(node) {
			const template = node.parent.parent;
			const vBindKeyOnTemplate = require_index.default.getDirective(template, "bind", "key");
			if (vBindKeyOnTemplate && isUsingIterationVar(node, vBindKeyOnTemplate)) return;
			for (const child of template.children) {
				if (!require_index.default.isVElement(child) || require_index.default.hasDirective(child, "if") || require_index.default.hasDirective(child, "else-if") || require_index.default.hasDirective(child, "else") || require_index.default.hasDirective(child, "for")) continue;
				const vBindKeyOnChild = require_index.default.getDirective(child, "bind", "key");
				if (vBindKeyOnChild && isUsingIterationVar(node, vBindKeyOnChild)) context.report({
					node: vBindKeyOnChild,
					loc: vBindKeyOnChild.loc,
					messageId: "vForTemplateKeyPlacement"
				});
			}
		} });
	}
};
exports.default = no_v_for_template_key_on_child_default;
