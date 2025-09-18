# Features - Specific Demonstrations

This directory contains focused examples demonstrating specific Cadwyn-Go features and capabilities.

## 📁 Available Features

### `/major_minor_versioning/`
**NEW!** Semantic versioning with major.minor format (no patch required).

Perfect for API versioning where you want semantic meaning:
- **Major versions** (1.0 → 2.0): Breaking changes
- **Minor versions** (1.0 → 1.1): New features, backward compatible

```bash
cd features/major_minor_versioning
go run main.go
```

**Key Benefits:**
- ✅ Semantic clarity: Major = breaking, Minor = new features
- ✅ Simpler version numbers (no patch needed for APIs)
- ✅ Clear backward compatibility expectations
- ✅ Automatic migration between all versions

**Example Usage:**
```go
app, err := cadwyn.NewBuilder().
    WithSemverVersions("1.0", "1.1", "2.0", "2.1").
    WithVersionChanges(
        NewV1_0ToV1_1Change(), // Minor: new features
        NewV1_1ToV2_0Change(), // Major: breaking changes
        NewV2_0ToV2_1Change(), // Minor: new features
    ).
    Build()
```

## 🔮 Future Features

More specific feature demonstrations will be added here:

- **Date-based versioning patterns**
- **Custom version detection strategies**
- **Advanced schema analysis**
- **Performance optimization techniques**
- **Integration patterns with popular frameworks**

## 📚 How to Use

Each feature directory contains:
- **`main.go`** - Runnable example
- **`README.md`** - Detailed explanation
- **Real-world use cases** and best practices

## 🎯 When to Use Feature Examples

- **Learning specific capabilities** of Cadwyn-Go
- **Implementing particular patterns** in your application
- **Understanding advanced use cases** beyond basic versioning
- **Exploring cutting-edge features** and techniques

## 🚀 Getting Started

1. **Start with basics**: `cd ../basic`
2. **Learn production patterns**: `cd ../intermediate`  
3. **Master advanced techniques**: `cd ../advanced`
4. **Explore specific features**: Choose from this directory

Each feature example is self-contained and can be run independently!
