const require_index = require("../utils/index.js");
// lib/rules/valid-template-root.ts
/**
* @author Toru Nagashima
* @copyright 2017 Toru Nagashima. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var valid_template_root_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce valid template root",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/valid-template-root.html"
		},
		fixable: null,
		schema: [],
		messages: {
			emptySrc: "The template root with 'src' attribute is required to be empty.",
			noChild: "The template requires child element."
		}
	},
	create(context) {
		const sourceCode = context.sourceCode;
		return { Program(program) {
			const element = program.templateBody;
			if (element == null) return;
			const hasSrc = require_index.default.hasAttribute(element, "src");
			const rootElements = [];
			for (const child of element.children) if (sourceCode.getText(child).trim() !== "") rootElements.push(child);
			if (hasSrc && rootElements.length > 0) for (const element of rootElements) context.report({
				node: element,
				loc: element.loc,
				messageId: "emptySrc"
			});
			else if (!hasSrc && rootElements.length === 0) context.report({
				node: element,
				loc: element.loc,
				messageId: "noChild"
			});
		} };
	}
};
exports.default = valid_template_root_default;
