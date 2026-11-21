const require_index = require("../utils/index.js");
// lib/rules/no-template-key.ts
/**
* @author Toru Nagashima
* @copyright 2016 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var no_template_key_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow `key` attribute on `<template>`",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-template-key.html"
		},
		fixable: null,
		schema: [],
		messages: { disallow: "'<template>' cannot be keyed. Place the key on real elements instead." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement[name='template']"(node) {
			const keyNode = require_index.default.getAttribute(node, "key") || require_index.default.getDirective(node, "bind", "key");
			if (!keyNode || require_index.default.hasDirective(node, "for")) return;
			context.report({
				node: keyNode,
				loc: keyNode.loc,
				messageId: "disallow"
			});
		} });
	}
};
exports.default = no_template_key_default;
