const require_index = require("../utils/index.js");
// lib/rules/no-export-in-script-setup.ts
var no_export_in_script_setup_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow `export` in `<script setup>`",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-export-in-script-setup.html"
		},
		fixable: null,
		schema: [],
		messages: { forbidden: "`<script setup>` cannot contain ES module exports." }
	},
	create(context) {
		function verify(node, loc) {
			const tsNode = node;
			if (tsNode.exportKind === "type" || tsNode.type === "ExportNamedDeclaration" && tsNode.specifiers.length > 0 && tsNode.specifiers.every((spec) => spec.exportKind === "type")) return;
			context.report({
				node,
				loc,
				messageId: "forbidden"
			});
		}
		return require_index.default.defineScriptSetupVisitor(context, {
			ExportAllDeclaration: (node) => verify(node, node.loc),
			ExportDefaultDeclaration: (node) => verify(node, node.loc),
			ExportNamedDeclaration: (node) => {
				verify(node, node.declaration ? context.sourceCode.getFirstToken(node).loc : node.loc);
			}
		});
	}
};
exports.default = no_export_in_script_setup_default;
