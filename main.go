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
	var localFilePath, remoteFileName, authCode, refreshToken string
	var initConfig, auth, refresh bool
	var authPort int

	flag.StringVar(&localFilePath, "file", "", "要上传的本地文件路径")
	flag.StringVar(&remoteFileName, "name", "", "上传到网盘的文件名（可选，默认使用本地文件名）")
	flag.StringVar(&authCode, "code", "", "授权码（用于获取access_token）")
	flag.StringVar(&refreshToken, "refresh", "", "刷新token")
	flag.BoolVar(&initConfig, "init", false, "初始化配置文件")
	flag.BoolVar(&auth, "auth", false, "启动授权流程")
	flag.BoolVar(&refresh, "refresh-token", false, "使用refresh_token刷新access_token")
	flag.IntVar(&authPort, "port", 8080, "授权回调服务器端口")
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
	if localFilePath == "" {
		fmt.Println("使用方法:")
		fmt.Println("  初始化配置: ./bddisk_uploader -init")
		fmt.Println("  授权登录: ./bddisk_uploader -auth")
		fmt.Println("  手动授权: ./bddisk_uploader -code <授权码>")
		fmt.Println("  刷新token: ./bddisk_uploader -refresh-token")
		fmt.Println("  上传文件: ./bddisk_uploader -file <本地文件路径> [-name <远程文件名>]")
		os.Exit(1)
	}

	// 检查文件是否存在
	if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
		fmt.Printf("文件不存在: %s\n", localFilePath)
		os.Exit(1)
	}

	// 如果没有指定远程文件名，使用本地文件名
	if remoteFileName == "" {
		remoteFileName = filepath.Base(localFilePath)
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

	// 上传文件
	fmt.Printf("开始上传文件: %s -> %s\n", localFilePath, remoteFileName)
	if err := uploadFile(config, localFilePath, remoteFileName); err != nil {
		fmt.Printf("上传失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("上传成功！")
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