const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-functional-template.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_functional_template_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated the `functional` template (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-functional-template.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "The `functional` template are deprecated." }
	},
	create(context) {
		return { Program(program) {
			const element = program.templateBody;
			if (element == null) return;
			const functional = require_index.default.getAttribute(element, "functional");
			if (functional) context.report({
				node: functional,
				messageId: "unexpected"
			});
		} };
	}
};
exports.default = no_deprecated_functional_template_default;
