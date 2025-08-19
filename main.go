package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"icode.baidu.com/baidu/xpan/go-sdk/xpan/upload"
)

const (
	ChunkSize = 4 * 1024 * 1024 // 4MB分片大小
	ConfigFile = "config.json"
)

type Config struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time  `json:"expires_at,omitempty"`
	AppPath      string      `json:"app_path"` // 应用路径前缀，如 "/apps/your_app_name/"
	OAuth        *OAuthConfig `json:"oauth,omitempty"`
}

// 加载配置文件
func loadConfig() (*Config, error) {
	configData, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	if config.AccessToken == "" {
		return nil, fmt.Errorf("配置文件中缺少access_token")
	}

	if config.AppPath == "" {
		config.AppPath = "/apps/baidu_netdisk_uploader/"
	}

	return &config, nil
}

// 创建默认配置文件
func createDefaultConfig() error {
	config := Config{
		AccessToken: "your_access_token_here",
		AppPath:     "/apps/baidu_netdisk_uploader/",
		OAuth: &OAuthConfig{
			ClientID:     "your_app_key_here",
			ClientSecret: "your_secret_key_here",
			RedirectURI:  DefaultRedirectURI,
			Scope:        DefaultScope,
		},
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigFile, configData, 0644)
}

// 计算文件分片的MD5值
func calculateFileMD5Chunks(filePath string) ([]string, uint64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}

	fileSize := uint64(fileInfo.Size())
	var md5List []string
	buffer := make([]byte, ChunkSize)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, 0, err
		}
		if n == 0 {
			break
		}

		hash := md5.Sum(buffer[:n])
		md5List = append(md5List, hex.EncodeToString(hash[:]))
	}

	return md5List, fileSize, nil
}

// 创建文件分片
func createFileChunks(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunkFiles []string
	buffer := make([]byte, ChunkSize)
	chunkIndex := 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		chunkFileName := fmt.Sprintf("%s.chunk.%d", filePath, chunkIndex)
		chunkFile, err := os.Create(chunkFileName)
		if err != nil {
			return nil, err
		}

		_, err = chunkFile.Write(buffer[:n])
		chunkFile.Close()
		if err != nil {
			return nil, err
		}

		chunkFiles = append(chunkFiles, chunkFileName)
		chunkIndex++
	}

	return chunkFiles, nil
}

// 清理临时分片文件
func cleanupChunks(chunkFiles []string) {
	for _, chunkFile := range chunkFiles {
		os.Remove(chunkFile)
	}
}

// 上传文件到百度网盘
func uploadFile(config *Config, localFilePath, remoteFileName string) error {
	// 计算文件MD5分片
	fmt.Printf("正在计算文件MD5分片...")
	md5List, fileSize, err := calculateFileMD5Chunks(localFilePath)
	if err != nil {
		return fmt.Errorf("计算文件MD5失败: %v", err)
	}
	fmt.Printf("完成，文件大小: %d 字节，分片数: %d\n", fileSize, len(md5List))

	// 构建远程路径
	remotePath := filepath.Join(config.AppPath, remoteFileName)
	remotePath = strings.ReplaceAll(remotePath, "\\", "/") // 确保使用Unix风格路径

	// 1. Precreate - 预创建文件
	fmt.Printf("正在预创建文件...")
	precreateArg := upload.NewPrecreateArg(remotePath, fileSize, md5List)
	precreateResult, err := upload.Precreate(config.AccessToken, precreateArg)
	if err != nil {
		return fmt.Errorf("预创建文件失败: %v", err)
	}
	fmt.Printf("完成，上传ID: %s\n", precreateResult.UploadId)

	if precreateResult.ReturnType == 2 {
		fmt.Println("文件已存在，无需重复上传")
		return nil
	}

	// 创建临时分片文件
	fmt.Printf("正在创建文件分片...")
	chunkFiles, err := createFileChunks(localFilePath)
	if err != nil {
		return fmt.Errorf("创建文件分片失败: %v", err)
	}
	defer cleanupChunks(chunkFiles)
	fmt.Printf("完成，共创建 %d 个分片\n", len(chunkFiles))

	// 2. Upload - 上传需要的分片
	for _, partSeq := range precreateResult.BlockList {
		if partSeq >= len(chunkFiles) {
			return fmt.Errorf("分片序号 %d 超出范围", partSeq)
		}

		fmt.Printf("正在上传分片 %d/%d...", partSeq+1, len(md5List))
		uploadArg := upload.NewUploadArg(
			precreateResult.UploadId,
			remotePath,
			chunkFiles[partSeq],
			partSeq,
		)

		uploadResult, err := upload.Upload(config.AccessToken, uploadArg)
		if err != nil {
			return fmt.Errorf("上传分片 %d 失败: %v", partSeq, err)
		}
		fmt.Printf("完成，MD5: %s\n", uploadResult.Md5)
	}

	// 3. Create - 创建文件
	fmt.Printf("正在合并文件...")
	createArg := upload.NewCreateArg(precreateResult.UploadId, remotePath, fileSize, md5List)
	createResult, err := upload.Create(config.AccessToken, createArg)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}

	if createResult.Errno != 0 {
		return fmt.Errorf("创建文件失败，错误码: %d", createResult.Errno)
	}

	fmt.Printf("完成！文件已成功上传到: %s\n", createResult.Path)
	return nil
}

func main() {
	var localFilePath, localFolderPath, remoteFileName, authCode, refreshToken, excludePatterns string
	var initConfig, auth, refresh, keepStructure bool
	var authPort, maxConcurrent int

	flag.StringVar(&localFilePath, "file", "", "要上传的本地文件路径")
	flag.StringVar(&localFolderPath, "folder", "", "要上传的本地文件夹路径")
	flag.StringVar(&remoteFileName, "name", "", "上传到网盘的文件名（可选，默认使用本地文件名）")
	flag.StringVar(&excludePatterns, "exclude", "", "要排除的文件模式，用逗号分隔（如：*.tmp,*.log,.DS_Store）")
	flag.StringVar(&authCode, "code", "", "授权码（用于获取access_token）")
	flag.StringVar(&refreshToken, "refresh", "", "刷新token")
	flag.BoolVar(&initConfig, "init", false, "初始化配置文件")
	flag.BoolVar(&auth, "auth", false, "启动授权流程")
	flag.BoolVar(&refresh, "refresh-token", false, "使用refresh_token刷新access_token")
	flag.BoolVar(&keepStructure, "keep-structure", true, "保持文件夹结构（默认启用）")
	flag.IntVar(&authPort, "port", 8080, "授权回调服务器端口")
	flag.IntVar(&maxConcurrent, "concurrent", 3, "最大并发上传数（默认3）")
	flag.Parse()

	// 初始化配置文件
	if initConfig {
		if err := createDefaultConfig(); err != nil {
			fmt.Printf("创建配置文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已创建配置文件 %s\n", ConfigFile)
		fmt.Println("请编辑配置文件中的以下信息:")
		fmt.Println("  - client_id: 您的App Key")
		fmt.Println("  - client_secret: 您的Secret Key")
		fmt.Println("  - app_path: 文件上传路径前缀")
		fmt.Println("然后运行: ./bddisk_uploader -auth 进行授权")
		return
	}

	// 处理授权流程
	if auth {
		config, err := loadConfigForAuth()
		if err != nil {
			fmt.Printf("加载配置失败: %v\n", err)
			os.Exit(1)
		}

		tokenResp, err := startAuthServer(config.OAuth, authPort)
		if err != nil {
			fmt.Printf("授权失败: %v\n", err)
			os.Exit(1)
		}

		// 保存token到配置文件
		if err := saveTokenToConfig(tokenResp); err != nil {
			fmt.Printf("保存token失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("授权成功！access_token已保存到配置文件")
		return
	}

	// 手动使用授权码获取token
	if authCode != "" {
		config, err := loadConfigForAuth()
		if err != nil {
			fmt.Printf("加载配置失败: %v\n", err)
			os.Exit(1)
		}

		tokenResp, err := getAccessToken(config.OAuth, authCode)
		if err != nil {
			fmt.Printf("获取token失败: %v\n", err)
			os.Exit(1)
		}

		if err := saveTokenToConfig(tokenResp); err != nil {
			fmt.Printf("保存token失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("access_token获取成功并已保存！")
		return
	}

	// 刷新access_token
	if refresh {
		config, err := loadConfigForAuth()
		if err != nil {
			fmt.Printf("加载配置失败: %v\n", err)
			os.Exit(1)
		}

		if config.RefreshToken == "" {
			fmt.Println("配置文件中没有refresh_token，请重新授权")
			os.Exit(1)
		}

		tokenResp, err := refreshAccessToken(config.OAuth, config.RefreshToken)
		if err != nil {
			fmt.Printf("刷新token失败: %v\n", err)
			os.Exit(1)
		}

		if err := saveTokenToConfig(tokenResp); err != nil {
			fmt.Printf("保存token失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("access_token刷新成功！")
		return
	}

	// 检查参数
	if localFilePath == "" && localFolderPath == "" {
		fmt.Println("使用方法:")
		fmt.Println("  初始化配置: ./bddisk_uploader -init")
		fmt.Println("  授权登录: ./bddisk_uploader -auth")
		fmt.Println("  手动授权: ./bddisk_uploader -code <授权码>")
		fmt.Println("  刷新token: ./bddisk_uploader -refresh-token")
		fmt.Println("  上传文件: ./bddisk_uploader -file <本地文件路径> [-name <远程文件名>]")
		fmt.Println("  上传文件夹: ./bddisk_uploader -folder <本地文件夹路径> [选项]")
		fmt.Println("")
		fmt.Println("文件夹上传选项:")
		fmt.Println("  -exclude <模式>        排除文件模式，逗号分隔")
		fmt.Println("  -keep-structure       保持文件夹结构（默认启用）")
		fmt.Println("  -concurrent <数量>     最大并发上传数（默认3）")
		os.Exit(1)
	}

	// 检查互斥参数
	if localFilePath != "" && localFolderPath != "" {
		fmt.Println("错误: -file 和 -folder 参数不能同时使用")
		os.Exit(1)
	}

	// 检查文件或文件夹是否存在
	var targetPath string
	var isFolder bool
	
	if localFolderPath != "" {
		targetPath = localFolderPath
		isFolder = true
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			fmt.Printf("文件夹不存在: %s\n", targetPath)
			os.Exit(1)
		}
		// 检查是否为目录
		if fileInfo, err := os.Stat(targetPath); err == nil && !fileInfo.IsDir() {
			fmt.Printf("错误: %s 不是一个文件夹\n", targetPath)
			os.Exit(1)
		}
	} else {
		targetPath = localFilePath
		isFolder = false
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			fmt.Printf("文件不存在: %s\n", targetPath)
			os.Exit(1)
		}
		// 如果没有指定远程文件名，使用本地文件名
		if remoteFileName == "" {
			remoteFileName = filepath.Base(targetPath)
		}
	}

	// 加载配置
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("配置错误: %v\n", err)
		fmt.Printf("请先运行: ./bddisk_uploader -init 来创建配置文件\n")
		os.Exit(1)
	}

	// 检查token是否过期
	if config.ExpiresAt != nil && time.Now().After(*config.ExpiresAt) {
		fmt.Println("access_token已过期，尝试自动刷新...")
		if config.RefreshToken != "" && config.OAuth != nil {
			tokenResp, err := refreshAccessToken(config.OAuth, config.RefreshToken)
			if err != nil {
				fmt.Printf("自动刷新token失败: %v\n", err)
				fmt.Println("请重新授权: ./bddisk_uploader -auth")
				os.Exit(1)
			}
			if err := saveTokenToConfig(tokenResp); err != nil {
				fmt.Printf("保存新token失败: %v\n", err)
				os.Exit(1)
			}
			config.AccessToken = tokenResp.AccessToken
			fmt.Println("access_token已自动刷新")
		} else {
			fmt.Println("无法自动刷新token，请重新授权: ./bddisk_uploader -auth")
			os.Exit(1)
		}
	}

	// 上传文件或文件夹
	if isFolder {
		// 上传文件夹
		excludeList := parseExcludePatterns(excludePatterns)
		fmt.Printf("开始上传文件夹: %s\n", targetPath)
		if err := uploadFolder(config, targetPath, excludeList, keepStructure, maxConcurrent); err != nil {
			fmt.Printf("上传失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("文件夹上传完成！")
	} else {
		// 上传单个文件
		fmt.Printf("开始上传文件: %s -> %s\n", targetPath, remoteFileName)
		if err := uploadFile(config, targetPath, remoteFileName); err != nil {
			fmt.Printf("上传失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("上传成功！")
	}
}

// 加载配置用于授权（不要求access_token存在）
func loadConfigForAuth() (*Config, error) {
	configData, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	if config.OAuth == nil {
		return nil, fmt.Errorf("配置文件中缺少OAuth信息")
	}

	if config.OAuth.ClientID == "" || config.OAuth.ClientID == "your_app_key_here" {
		return nil, fmt.Errorf("请先在配置文件中设置正确的client_id (App Key)")
	}

	if config.OAuth.ClientSecret == "" || config.OAuth.ClientSecret == "your_secret_key_here" {
		return nil, fmt.Errorf("请先在配置文件中设置正确的client_secret (Secret Key)")
	}

	if config.AppPath == "" {
		config.AppPath = "/apps/baidu_netdisk_uploader/"
	}

	return &config, nil
}

// 保存token到配置文件
func saveTokenToConfig(tokenResp *TokenResponse) error {
	// 读取现有配置
	config, err := loadConfigForAuth()
	if err != nil {
		return err
	}

	// 更新token信息
	config.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		config.RefreshToken = tokenResp.RefreshToken
	}
	
	// 计算过期时间
	if tokenResp.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		config.ExpiresAt = &expiresAt
	}

	// 保存到文件
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	return os.WriteFile(ConfigFile, configData, 0644)
}

// 文件信息结构
type FileInfo struct {
	LocalPath  string
	RemotePath string
	Size       int64
	ModTime    time.Time
}

// 上传结果统计
type UploadStats struct {
	TotalFiles    int64
	UploadedFiles int64
	FailedFiles   int64
	TotalSize     int64
	UploadedSize  int64
	StartTime     time.Time
}

// 解析排除模式
func parseExcludePatterns(patterns string) []string {
	if patterns == "" {
		return []string{}
	}
	
	parts := strings.Split(patterns, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// 检查文件是否应该被排除
func shouldExcludeFile(filePath string, excludePatterns []string) bool {
	fileName := filepath.Base(filePath)
	
	// 默认排除的文件
	defaultExcludes := []string{
		".DS_Store",
		"Thumbs.db",
		".git",
		".svn",
		".hg",
		"node_modules",
		"*.tmp",
		"*.temp",
		"*~",
	}
	
	allPatterns := append(excludePatterns, defaultExcludes...)
	
	for _, pattern := range allPatterns {
		if matched, _ := filepath.Match(pattern, fileName); matched {
			return true
		}
		// 也检查完整路径
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true
		}
	}
	return false
}

// 收集文件夹中的所有文件
func collectFiles(folderPath string, excludePatterns []string, keepStructure bool) ([]FileInfo, error) {
	var files []FileInfo
	
	// 获取文件夹名称，用于保持完整的目录结构
	folderName := filepath.Base(folderPath)
	
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("警告: 访问文件失败 %s: %v\n", path, err)
			return nil // 继续处理其他文件
		}
		
		// 跳过目录
		if info.IsDir() {
			return nil
		}
		
		// 检查是否应该排除
		if shouldExcludeFile(path, excludePatterns) {
			fmt.Printf("跳过文件: %s\n", path)
			return nil
		}
		
		// 计算远程路径
		var remotePath string
		if keepStructure {
			// 保持目录结构，包含最外层文件夹名
			relPath, err := filepath.Rel(folderPath, path)
			if err != nil {
				return fmt.Errorf("计算相对路径失败: %v", err)
			}
			// 将文件夹名称作为根目录
			remotePath = filepath.Join(folderName, relPath)
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")
		} else {
			// 平铺所有文件到文件夹根目录
			remotePath = filepath.Join(folderName, info.Name())
			remotePath = strings.ReplaceAll(remotePath, "\\", "/")
		}
		
		files = append(files, FileInfo{
			LocalPath:  path,
			RemotePath: remotePath,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
		})
		
		return nil
	})
	
	return files, err
}

// 上传单个文件（用于并发上传）
func uploadSingleFile(config *Config, fileInfo FileInfo, stats *UploadStats, wg *sync.WaitGroup, semaphore chan struct{}) {
	defer wg.Done()
	defer func() { <-semaphore }() // 释放信号量
	
	fmt.Printf("[%d/%d] 上传: %s\n", 
		atomic.LoadInt64(&stats.UploadedFiles)+atomic.LoadInt64(&stats.FailedFiles)+1, 
		stats.TotalFiles, 
		fileInfo.RemotePath)
	
	err := uploadFile(config, fileInfo.LocalPath, fileInfo.RemotePath)
	if err != nil {
		atomic.AddInt64(&stats.FailedFiles, 1)
		fmt.Printf("❌ 上传失败: %s - %v\n", fileInfo.RemotePath, err)
	} else {
		atomic.AddInt64(&stats.UploadedFiles, 1)
		atomic.AddInt64(&stats.UploadedSize, fileInfo.Size)
		fmt.Printf("✅ 上传成功: %s\n", fileInfo.RemotePath)
	}
}

// 格式化文件大小
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// 格式化持续时间
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// 上传文件夹
func uploadFolder(config *Config, folderPath string, excludePatterns []string, keepStructure bool, maxConcurrent int) error {
	// 收集所有需要上传的文件
	fmt.Println("正在扫描文件...")
	files, err := collectFiles(folderPath, excludePatterns, keepStructure)
	if err != nil {
		return fmt.Errorf("收集文件失败: %v", err)
	}
	
	if len(files) == 0 {
		fmt.Println("没有找到需要上传的文件")
		return nil
	}
	
	// 计算总大小
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}
	
	// 初始化统计信息
	stats := &UploadStats{
		TotalFiles: int64(len(files)),
		TotalSize:  totalSize,
		StartTime:  time.Now(),
	}
	
	fmt.Printf("发现 %d 个文件，总大小: %s\n", len(files), formatFileSize(totalSize))
	fmt.Printf("开始并发上传 (最大并发数: %d)...\n\n", maxConcurrent)
	
	// 创建信号量控制并发数
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	
	// 启动进度监控
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				uploaded := atomic.LoadInt64(&stats.UploadedFiles)
				failed := atomic.LoadInt64(&stats.FailedFiles)
				uploadedSize := atomic.LoadInt64(&stats.UploadedSize)
				
				progress := float64(uploaded+failed) / float64(stats.TotalFiles) * 100
				elapsed := time.Since(stats.StartTime)
				
				fmt.Printf("\n📊 进度报告: %.1f%% (%d/%d) | 成功: %d | 失败: %d | 已传输: %s/%s | 耗时: %s\n\n",
					progress, uploaded+failed, stats.TotalFiles, uploaded, failed,
					formatFileSize(uploadedSize), formatFileSize(totalSize), formatDuration(elapsed))
			case <-done:
				return
			}
		}
	}()
	
	// 并发上传文件
	for _, file := range files {
		semaphore <- struct{}{} // 获取信号量
		wg.Add(1)
		go uploadSingleFile(config, file, stats, &wg, semaphore)
	}
	
	// 等待所有上传完成
	wg.Wait()
	done <- true
	
	// 显示最终统计
	elapsed := time.Since(stats.StartTime)
	uploaded := atomic.LoadInt64(&stats.UploadedFiles)
	failed := atomic.LoadInt64(&stats.FailedFiles)
	uploadedSize := atomic.LoadInt64(&stats.UploadedSize)
	
	fmt.Printf("\n🎉 上传完成!\n")
	fmt.Printf("总文件数: %d\n", stats.TotalFiles)
	fmt.Printf("成功上传: %d\n", uploaded)
	fmt.Printf("失败文件: %d\n", failed)
	fmt.Printf("传输大小: %s / %s\n", formatFileSize(uploadedSize), formatFileSize(totalSize))
	fmt.Printf("总耗时: %s\n", formatDuration(elapsed))
	
	if uploaded > 0 {
		avgSpeed := float64(uploadedSize) / elapsed.Seconds()
		fmt.Printf("平均速度: %s/s\n", formatFileSize(int64(avgSpeed)))
	}
	
	if failed > 0 {
		return fmt.Errorf("有 %d 个文件上传失败", failed)
	}
	
	return nil
}