const require_typescript = require("./typescript.js");
// lib/utils/ts-utils/ts-types.ts
/**
* Get TypeScript parser services.
*/
function getTSParserServices(context) {
	const sourceCode = context.sourceCode;
	const tsNodeMap = sourceCode.parserServices.esTreeNodeToTSNodeMap;
	if (!tsNodeMap) return null;
	const checker = sourceCode.parserServices.hasFullTypeInformation !== false && sourceCode.parserServices.program && sourceCode.parserServices.program.getTypeChecker() || null;
	if (!checker) return null;
	const ts = require_typescript.getTypeScript();
	if (!ts) return null;
	return {
		ts,
		tsNodeMap,
		checker
	};
}
/**
* Get all props by looking at all component's properties
*/
function getComponentPropsFromTypeDefineTypes(context, propsNode) {
	const services = getTSParserServices(context);
	const tsNode = services && services.tsNodeMap.get(propsNode);
	const type = tsNode && services.checker.getTypeAtLocation(tsNode);
	if (!type || require_typescript.isAny(type) || require_typescript.isUnknown(type) || require_typescript.isNever(type) || require_typescript.isNull(type)) return [{
		type: "unknown",
		propName: null,
		node: propsNode
	}];
	return [...extractRuntimeProps(type, tsNode, propsNode, services)];
}
/**
* Get all emits by looking at all component's properties
*/
function getComponentEmitsFromTypeDefineTypes(context, emitsNode) {
	const services = getTSParserServices(context);
	const tsNode = services && services.tsNodeMap.get(emitsNode);
	const type = tsNode && services.checker.getTypeAtLocation(tsNode);
	if (!type || require_typescript.isAny(type) || require_typescript.isUnknown(type) || require_typescript.isNever(type) || require_typescript.isNull(type)) return [{
		type: "unknown",
		emitName: null,
		node: emitsNode
	}];
	return [...extractRuntimeEmits(type, tsNode, emitsNode, services)];
}
/**
* Get all slots by looking at all component's properties
*/
function getComponentSlotsFromTypeDefineTypes(context, slotsNode) {
	const services = getTSParserServices(context);
	const tsNode = services && services.tsNodeMap.get(slotsNode);
	const type = tsNode && services.checker.getTypeAtLocation(tsNode);
	if (!type || require_typescript.isAny(type) || require_typescript.isUnknown(type) || require_typescript.isNever(type) || require_typescript.isNull(type)) return [{
		type: "unknown",
		slotName: null,
		node: slotsNode
	}];
	return [...extractRuntimeSlots(type, slotsNode)];
}
function inferRuntimeTypeFromTypeNode(context, node) {
	const services = getTSParserServices(context);
	const tsNode = services && services.tsNodeMap.get(node);
	const type = tsNode && services.checker.getTypeAtLocation(tsNode);
	return type ? inferRuntimeTypeInternal(type, services) : ["null"];
}
function* extractRuntimeProps(type, tsNode, propsNode, services) {
	const { ts, checker } = services;
	for (const property of type.getProperties()) {
		const isOptional = (property.flags & ts.SymbolFlags.Optional) !== 0;
		const name = property.getName();
		const type = checker.getTypeOfSymbolAtLocation(property, tsNode);
		yield {
			type: "infer-type",
			propName: name,
			required: !isOptional,
			node: propsNode,
			types: inferRuntimeTypeInternal(type, services)
		};
	}
}
function inferRuntimeTypeInternal(type, services) {
	const { checker } = services;
	const types = /* @__PURE__ */ new Set();
	if (type.isTypeParameter()) {
		const constraint = type.getConstraint();
		if (constraint) for (const t of inferRuntimeTypeInternal(constraint, services)) types.add(t);
		return [...types];
	}
	const targetTypes = iterateTypes(checker.getNonNullableType(type));
	for (const targetType of targetTypes) if (require_typescript.isAny(targetType) || require_typescript.isUnknown(targetType) || require_typescript.isNever(targetType) || require_typescript.isNull(targetType)) types.add("null");
	else if (require_typescript.isStringLike(targetType)) types.add("String");
	else if (require_typescript.isNumberLike(targetType) || require_typescript.isBigIntLike(targetType)) types.add("Number");
	else if (require_typescript.isBooleanLike(targetType)) types.add("Boolean");
	else if (require_typescript.isFunction(targetType)) types.add("Function");
	else if (require_typescript.isArrayLikeObject(targetType) || targetType.isClassOrInterface() && ["Array", "ReadonlyArray"].includes(checker.getFullyQualifiedName(targetType.symbol))) types.add("Array");
	else if (require_typescript.isObject(targetType)) types.add("Object");
	if (types.size === 0) types.add("null");
	return [...types];
}
function* extractRuntimeEmits(type, tsNode, emitsNode, services) {
	const { checker } = services;
	if (require_typescript.isFunction(type)) for (const signature of type.getCallSignatures()) {
		const param = signature.getParameters()[0];
		if (!param) {
			yield {
				type: "unknown",
				emitName: null,
				node: emitsNode
			};
			continue;
		}
		const type = checker.getTypeOfSymbolAtLocation(param, tsNode);
		for (const targetType of iterateTypes(type)) yield targetType.isStringLiteral() ? {
			type: "infer-type",
			emitName: targetType.value,
			node: emitsNode
		} : {
			type: "unknown",
			emitName: null,
			node: emitsNode
		};
	}
	else if (require_typescript.isObject(type)) for (const property of type.getProperties()) yield {
		type: "infer-type",
		emitName: property.getName(),
		node: emitsNode
	};
	else yield {
		type: "unknown",
		emitName: null,
		node: emitsNode
	};
}
function* extractRuntimeSlots(type, slotsNode) {
	for (const property of type.getProperties()) yield {
		type: "infer-type",
		slotName: property.getName(),
		node: slotsNode
	};
}
function* iterateTypes(type) {
	if (require_typescript.isReferenceObject(type) && type.target !== type) yield* iterateTypes(type.target);
	else if (type.isUnion() && !require_typescript.isBooleanLike(type)) for (const t of type.types) yield* iterateTypes(t);
	else yield type;
}
function unionConstituents(type) {
	return type.isUnion() ? type.types : [type];
}
function intersectionConstituents(type) {
	return type.isIntersection() ? type.types : [type];
}
function isTypeLiteralLike(type, ts) {
	return (type.flags & (ts.TypeFlags.Literal | ts.TypeFlags.Undefined | ts.TypeFlags.Null | ts.TypeFlags.UniqueESSymbol)) !== 0;
}
/**
* Check whether the given expression is typed as a Vue ref object,
* e.g. `Ref`, `ShallowRef` or `ComputedRef`.
*/
function isRefObjectType(context, node) {
	const services = getTSParserServices(context);
	const tsNode = services && services.tsNodeMap.get(node);
	if (!tsNode) return false;
	const { checker } = services;
	const type = checker.getTypeAtLocation(tsNode);
	return checker.getPropertiesOfType(type).some((property) => String(property.escapedName).startsWith("__@RefSymbol@"));
}
/**
* Check whether a switch statement's discriminant is a finite union of literal
* types and every union member is covered by a case clause.
* Does NOT verify that case bodies terminate — callers can use ESLint's code
* path analysis for that.
*/
function isExhaustiveSwitch(context, switchNode) {
	const services = getTSParserServices(context);
	if (!services) return false;
	const { ts, tsNodeMap, checker } = services;
	const discriminantTSNode = tsNodeMap.get(switchNode.discriminant);
	if (!discriminantTSNode) return false;
	let discriminantType = checker.getTypeAtLocation(discriminantTSNode);
	if (!discriminantType) return false;
	discriminantType = checker.getBaseConstraintOfType(discriminantType) ?? discriminantType;
	const caseTypes = /* @__PURE__ */ new Set();
	for (const switchCase of switchNode.cases) {
		if (switchCase.test === null) return true;
		const caseTSNode = tsNodeMap.get(switchCase.test);
		if (!caseTSNode) return false;
		let caseType = checker.getTypeAtLocation(caseTSNode);
		if (!caseType) return false;
		caseType = checker.getBaseConstraintOfType(caseType) ?? caseType;
		caseTypes.add(caseType);
	}
	const hasUndefinedCase = [...caseTypes].some((type) => (type.flags & ts.TypeFlags.Undefined) !== 0);
	for (const member of unionConstituents(discriminantType)) if (!intersectionConstituents(member).some((part) => isTypeLiteralLike(part, ts) && (caseTypes.has(part) || hasUndefinedCase && (part.flags & ts.TypeFlags.Undefined) !== 0))) return false;
	return true;
}
exports.getComponentEmitsFromTypeDefineTypes = getComponentEmitsFromTypeDefineTypes;
exports.getComponentPropsFromTypeDefineTypes = getComponentPropsFromTypeDefineTypes;
exports.getComponentSlotsFromTypeDefineTypes = getComponentSlotsFromTypeDefineTypes;
exports.inferRuntimeTypeFromTypeNode = inferRuntimeTypeFromTypeNode;
exports.isExhaustiveSwitch = isExhaustiveSwitch;
exports.isRefObjectType = isRefObjectType;
