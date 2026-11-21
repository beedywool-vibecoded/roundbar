const require_index = require("../utils/index.js");
// lib/rules/no-v-for-template-key.ts
/**
* @author Yosuke Ota
*/
var no_v_for_template_key_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow `key` attribute on `<template v-for>`",
			categories: ["vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-v-for-template-key.html"
		},
		fixable: null,
		deprecated: true,
		schema: [],
		messages: { disallow: "'<template v-for>' cannot be keyed. Place the key on real elements instead." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement[name='template'] > VStartTag > VAttribute[directive=true][key.name.name='for']"(node) {
			const element = node.parent.parent;
			const keyNode = require_index.default.getAttribute(element, "key") || require_index.default.getDirective(element, "bind", "key");
			if (keyNode) context.report({
				node: keyNode,
				loc: keyNode.loc,
				messageId: "disallow"
			});
		} });
	}
};
exports.default = no_v_for_template_key_default;
