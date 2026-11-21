const require_index = require("../utils/index.js");
// lib/rules/no-root-v-if.ts
/**
* @author Perry Song
* @copyright 2023 Perry Song. All rights reserved.
* See LICENSE file in root directory for full license.
*/
var no_root_v_if_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow `v-if` directives on root element",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-root-v-if.html"
		},
		fixable: null,
		schema: [],
		messages: { noRootVIf: "`v-if` should not be used on root element without `v-else`." }
	},
	create(context) {
		return { Program(program) {
			const element = program.templateBody;
			if (element == null) return;
			const rootElements = element.children.filter(require_index.default.isVElement);
			if (rootElements.length === 1 && require_index.default.hasDirective(rootElements[0], "if")) context.report({
				node: element,
				loc: element.loc,
				messageId: "noRootVIf"
			});
		} };
	}
};
exports.default = no_root_v_if_default;
